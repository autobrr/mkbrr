// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"fmt"
	"slices"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
)

// fieldAction tells applyMetadata what to do with one field.
type fieldAction uint8

const (
	keepField fieldAction = iota
	setField
	clearField
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
	WebSeeds     []string // nil keeps the current web seeds
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
	}

	if spec.WebSeeds != nil && !slices.Equal(mi.UrlList, spec.WebSeeds) {
		mi.UrlList = spec.WebSeeds
		changed = true
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
	}
	return false
}
