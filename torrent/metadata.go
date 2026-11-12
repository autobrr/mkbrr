// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
)

// fieldAction tells applyMetadata what to do with one field.
type fieldAction uint8

const (
	keepField fieldAction = iota
	setField
	clearField
	removeField // Trackers only: removes the trackers on the hosts in the value
)

// field is one metadata value and the action for it.
type field[T any] struct {
	action fieldAction
	value  T
}

func setTo[T any](v T) field[T] { return field[T]{action: setField, value: v} }

func cleared[T any]() field[T] { return field[T]{action: clearField} }

// metadataSpec is the metadata that create and modify write to a torrent.
// The zero value keeps every field.
type metadataSpec struct {
	Trackers     field[[]string]
	WebSeeds     field[[]string]
	Comment      field[string]
	CreatedBy    field[string]
	CreationDate field[int64]
	Name         field[string]
	Private      field[bool]
	Source       field[string]
	Entropy      fieldAction // set writes a new random value
}

// createdBy returns the "created by" value for an mkbrr version.
func createdBy(version string) string {
	if version == "" {
		version = "dev"
	}
	return fmt.Sprintf("mkbrr/%s (https://github.com/autobrr/mkbrr)", version)
}

// applyMetadata writes spec to mi. It reports a change only when a new value
// is different from the current value. Info keys that spec does not name stay
// as they are.
func applyMetadata(mi *metainfo.MetaInfo, spec metadataSpec) (bool, error) {
	changed := false

	switch spec.Trackers.action {
	case setField:
		trackers := spec.Trackers.value
		var announceList metainfo.AnnounceList
		if len(trackers) > 1 {
			announceList = make(metainfo.AnnounceList, len(trackers))
			for i, tracker := range trackers {
				announceList[i] = []string{tracker}
			}
		}
		var announce string
		if len(trackers) > 0 {
			announce = trackers[0]
		}
		if mi.Announce != announce || !slices.EqualFunc(mi.AnnounceList, announceList, slices.Equal) {
			mi.Announce = announce
			mi.AnnounceList = announceList
			changed = true
		}
	case clearField:
		if mi.Announce != "" || mi.AnnounceList != nil {
			mi.Announce = ""
			mi.AnnounceList = nil
			changed = true
		}
	case removeField:
		changed = removeTrackers(mi, spec.Trackers.value) || changed
	case keepField:
	}

	switch spec.WebSeeds.action {
	case setField:
		if !slices.Equal(mi.UrlList, spec.WebSeeds.value) {
			mi.UrlList = spec.WebSeeds.value
			changed = true
		}
	case clearField:
		if len(mi.UrlList) > 0 {
			mi.UrlList = nil
			changed = true
		}
	case keepField, removeField:
	}

	changed = applyValue(&mi.Comment, spec.Comment) || changed
	changed = applyValue(&mi.CreatedBy, spec.CreatedBy) || changed
	changed = applyValue(&mi.CreationDate, spec.CreationDate) || changed

	// edit the raw info map so that keys mkbrr does not know stay as they are
	info := make(map[string]any)
	if err := bencode.Unmarshal(mi.InfoBytes, &info); err != nil {
		return false, fmt.Errorf("could not unmarshal info: %w", err)
	}

	infoChanged := false
	private := field[int64]{action: spec.Private.action}
	if spec.Private.value {
		private.value = 1
	}
	infoChanged = applyInfoValue(info, "name", spec.Name) || infoChanged
	infoChanged = applyInfoValue(info, "private", private) || infoChanged
	infoChanged = applyInfoValue(info, "source", spec.Source) || infoChanged

	switch spec.Entropy {
	case setField:
		entropy, err := generateRandomString()
		if err != nil {
			return false, fmt.Errorf("could not generate entropy: %w", err)
		}
		info["entropy"] = entropy
		infoChanged = true
	case clearField:
		infoChanged = applyInfoValue(info, "entropy", cleared[string]()) || infoChanged
	case keepField, removeField:
	}

	if !infoChanged {
		return changed, nil
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return false, fmt.Errorf("could not marshal info: %w", err)
	}
	mi.InfoBytes = infoBytes
	return true, nil
}

// A removed main tracker becomes the first URL that remains.
func removeTrackers(mi *metainfo.MetaInfo, hosts []string) bool {
	removed := false
	var announceList metainfo.AnnounceList
	for _, tier := range mi.AnnounceList {
		kept := slices.DeleteFunc(slices.Clone(tier), func(u string) bool { return onHosts(u, hosts) })
		removed = removed || len(kept) < len(tier)
		if len(kept) > 0 {
			announceList = append(announceList, kept)
		}
	}
	announce := mi.Announce
	if onHosts(announce, hosts) {
		removed = true
		announce = ""
		if len(announceList) > 0 {
			announce = announceList[0][0]
		}
	}
	if !removed {
		return false
	}
	mi.Announce = announce
	mi.AnnounceList = announceList
	return true
}

func unmatchedHosts(mi *metainfo.MetaInfo, hosts []string) []string {
	urls := append([]string{mi.Announce}, slices.Concat(mi.AnnounceList...)...)
	var unmatched []string
	for _, host := range hosts {
		if !slices.ContainsFunc(urls, func(u string) bool { return onHosts(u, []string{host}) }) {
			unmatched = append(unmatched, host)
		}
	}
	return unmatched
}

// A host can also be a URL. The match never matches a subdomain.
func onHosts(tracker string, hosts []string) bool {
	h := hostOf(tracker)
	return h != "" && slices.ContainsFunc(hosts, func(host string) bool { return hostOf(host) == h })
}

func hostOf(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") {
		s = "//" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// applyValue writes f to *dst and reports whether *dst changed.
func applyValue[T comparable](dst *T, f field[T]) bool {
	var want T
	switch f.action {
	case setField:
		want = f.value
	case clearField:
	default:
		return false
	}
	if *dst == want {
		return false
	}
	*dst = want
	return true
}

// applyInfoValue writes f to info[key] and reports whether info changed.
// Clear removes the key, so a key with an empty value also counts as present.
func applyInfoValue[T comparable](info map[string]any, key string, f field[T]) bool {
	current, present := info[key]
	switch f.action {
	case setField:
		if present && current == any(f.value) {
			return false
		}
		info[key] = f.value
		return true
	case clearField:
		delete(info, key)
		return present
	case keepField, removeField:
	}
	return false
}
