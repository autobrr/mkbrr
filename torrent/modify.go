// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/autobrr/go-torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/preset"
)

// ModifySettings are the modify settings that a caller gives as overrides.
// The CLI and the GUI build this type. Each setting is keep, set, or clear:
//   - keep: the zero value. The preset applies if it sets the setting,
//     otherwise the torrent keeps its current value.
//   - set: a value.
//   - clear: an empty Comment or Source, or the No field of the setting.
//
// Only an override can clear a setting. A preset cannot clear.
// The flag tag names the CLI flag, and the preset tag names the preset key.
// A field with no preset tag has no preset key.
// TestSettingsParity in cmd checks both tags.
type ModifySettings struct {
	TrackerURLs    []string `json:"trackerUrls" flag:"tracker" preset:"trackers"`       // replaces the trackers; empty keeps
	NoTracker      bool     `json:"noTracker" flag:"no-tracker"`                        // clears the trackers
	RemoveTrackers []string `json:"removeTrackers" flag:"remove-tracker"`               // removes the trackers on these hosts or the hosts of these URLs
	WebSeeds       []string `json:"webSeeds" flag:"web-seed" preset:"webseeds"`         // replaces the web seeds; empty keeps
	NoWebSeed      bool     `json:"noWebSeed" flag:"no-web-seed"`                       // clears the web seeds
	Name           string   `json:"name" flag:"name"`                                   // empty keeps
	Comment        *string  `json:"comment" flag:"comment" preset:"comment"`            // empty clears
	Source         *string  `json:"source" flag:"source" preset:"source"`               // empty clears
	IsPrivate      *bool    `json:"isPrivate" flag:"private" preset:"private"`          // sets the private flag
	NoPrivate      bool     `json:"noPrivate" flag:"no-private"`                        // clears the private flag; wins over IsPrivate
	NoDate         bool     `json:"noDate" flag:"no-date" preset:"no_date"`             // clears the creation date
	NoCreator      bool     `json:"noCreator" flag:"no-creator" preset:"no_creator"`    // clears the creator
	Entropy        *bool    `json:"entropy" flag:"entropy" preset:"entropy"`            // true writes a new entropy value; false keeps, also over a preset
	NoEntropy      bool     `json:"noEntropy" flag:"no-entropy"`                        // clears the entropy field
	OutputDir      string   `json:"outputDir" flag:"output-dir" preset:"output_dir"`    // empty uses the preset output dir
	OutputPattern  string   `json:"outputPattern" flag:"output"`                        // custom output filename, without extension
	SkipPrefix     bool     `json:"skipPrefix" flag:"skip-prefix" preset:"skip_prefix"` // no filename prefix in the output filename
}

var errEntropyConflict = errors.New("cannot both add and remove entropy")

func (s ModifySettings) validate() error {
	if s.NoEntropy && s.Entropy != nil && *s.Entropy {
		return errEntropyConflict
	}
	conflicts := []struct {
		a, b       string
		hasA, hasB bool
	}{
		{"tracker", "no-tracker", len(s.TrackerURLs) > 0, s.NoTracker},
		{"tracker", "remove-tracker", len(s.TrackerURLs) > 0, len(s.RemoveTrackers) > 0},
		{"no-tracker", "remove-tracker", s.NoTracker, len(s.RemoveTrackers) > 0},
		{"web-seed", "no-web-seed", len(s.WebSeeds) > 0, s.NoWebSeed},
	}
	for _, c := range conflicts {
		if c.hasA && c.hasB {
			return fmt.Errorf("cannot use --%s with --%s", c.a, c.b)
		}
	}
	return nil
}

// ModifyOptions represents the options for modifying a torrent:
// the modify settings, the preset to apply, and the runtime fields.
type ModifyOptions struct {
	ModifySettings
	PresetName string
	PresetFile string
	Version    string
	DryRun     bool
	Verbose    bool
	Quiet      bool
}

// Result represents the result of modifying a torrent.
// Notices has a warning for each RemoveTrackers host that matched no tracker.
type Result struct {
	Error       error
	Path        string
	OutputPath  string
	WasModified bool
	Notices     []Notice
}

// LoadFromFile loads a torrent file from disk and returns a Torrent struct.
// The returned Torrent wraps the metainfo and provides additional functionality.
func LoadFromFile(path string) (*Torrent, error) {
	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not load torrent: %w", err)
	}
	return &Torrent{MetaInfo: mi}, nil
}

// ModifyTorrent modifies a single torrent file according to the given options.
// It can change trackers, comment, source, piece length, and other metadata.
// Returns a Result containing the operation outcome and output path.
func ModifyTorrent(path string, opts ModifyOptions) (*Result, error) {
	result := &Result{
		Path: path,
	}

	if err := opts.validate(); err != nil {
		result.Error = err
		return result, err
	}

	// load torrent file
	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		result.Error = fmt.Errorf("could not load torrent: %w", err)
		return result, result.Error
	}

	presetOpts, err := loadModifyPreset(opts)
	if err != nil {
		result.Error = err
		return result, result.Error
	}

	// read the original name before any change, for the output path
	info, err := mi.UnmarshalInfo()
	if err != nil {
		result.Error = fmt.Errorf("could not unmarshal info: %w", err)
		return result, result.Error
	}
	originalMetaInfoName := info.Name

	resolution := resolveModify(opts, presetOpts)
	spec := resolution.spec()
	if spec.Trackers.action == removeField {
		for _, host := range unmatchedHosts(mi, spec.Trackers.value) {
			result.Notices = append(result.Notices, Notice{Warn: true, Text: host + ": no matching tracker"})
		}
	}
	wasModified, err := applyMetadata(mi, spec)
	if err != nil {
		result.Error = err
		return result, result.Error
	}
	if spec.CreationDate.action == keepField && (wasModified || resolution.wantsDate() && mi.CreationDate == 0) {
		mi.CreationDate = time.Now().Unix()
		wasModified = true
	}

	if !wasModified {
		return result, nil
	}

	if opts.DryRun {
		result.WasModified = true
		return result, nil
	}

	metaInfoName := cmp.Or(opts.Name, originalMetaInfoName)

	basePath := path
	if opts.OutputPattern == "" && originalMetaInfoName != "" {
		basePath = originalMetaInfoName + ".torrent"
	}

	outputDir := resolution.OutputDir.Value

	// generate output path using the preset generating helper
	var trackerForOutput string
	if len(opts.TrackerURLs) > 0 {
		trackerForOutput = opts.TrackerURLs[0]
	} else {
		trackerForOutput = ""
	}
	outPath := preset.GenerateOutputPath(basePath, outputDir, opts.PresetName, opts.OutputPattern, trackerForOutput, metaInfoName, resolution.SkipPrefix.Value)
	result.OutputPath = outPath

	// ensure output directory exists if specified
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			result.Error = fmt.Errorf("could not create output directory: %w", err)
			return result, result.Error
		}
	}

	if err := writeTorrentOutput(outPath, 0o666, true, mi.Write); err != nil {
		result.Error = err
		return result, result.Error
	}

	result.WasModified = true
	return result, nil
}

// ProcessTorrents modifies multiple torrent files according to the given options.
// It processes each torrent file and returns the results for all operations.
// This function provides parallel processing for better performance with multiple files.
func ProcessTorrents(paths []string, opts ModifyOptions) ([]*Result, error) {
	if len(paths) == 0 {
		return nil, errors.New("no torrent files specified")
	}
	// fail once before the first file, not once per file
	if err := opts.validate(); err != nil {
		return nil, err
	}

	results := make([]*Result, 0, len(paths))
	for _, path := range paths {
		result, err := ModifyTorrent(path, opts)
		if err != nil {
			// continue processing other files even if one fails
			result.Error = err
		}
		results = append(results, result)
	}

	return results, nil
}
