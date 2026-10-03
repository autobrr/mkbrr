// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"cmp"
	"fmt"
	"os"
	"time"

	"github.com/autobrr/go-torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/preset"
)

// ModifyOptions represents the options for modifying a torrent,
// including both preset-related options and flag-based overrides.
type ModifyOptions struct {
	IsPrivate      *bool
	PieceLengthExp *uint
	MaxPieceLength *uint
	PresetName     string
	PresetFile     string
	Name           string
	OutputDir      string
	OutputPattern  string
	TrackerURLs    []string
	Comment        string
	Source         string
	Version        string
	WebSeeds       []string
	NoDate         bool
	NoCreator      bool
	DryRun         bool
	Verbose        bool
	Quiet          bool
	Entropy        *bool
	SkipPrefix     bool
	SourceSet      bool // true when --source flag was explicitly provided (allows empty string to clear)
	CommentSet     bool // true when --comment flag was explicitly provided (allows empty string to clear)
	RemovePrivate  bool // true when --no-private flag is provided (removes private field entirely)
}

// Result represents the result of modifying a torrent
type Result struct {
	Error       error
	Path        string
	OutputPath  string
	WasModified bool
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

	// load torrent file
	mi, err := metainfo.LoadFromFile(path)
	if err != nil {
		result.Error = fmt.Errorf("could not load torrent: %w", err)
		return result, result.Error
	}

	// load preset if specified
	var presetOpts *preset.Options
	if opts.PresetName != "" {
		presetPath, err := preset.FindPresetFile(opts.PresetFile)
		if err != nil {
			result.Error = fmt.Errorf("could not find preset file: %w", err)
			return result, result.Error
		}

		presets, err := preset.Load(presetPath)
		if err != nil {
			result.Error = fmt.Errorf("could not load presets: %w", err)
			return result, result.Error
		}

		presetOpts, err = presets.PresetValues(opts.PresetName)
		if err != nil {
			result.Error = fmt.Errorf("could not get preset: %w", err)
			return result, result.Error
		}
	}

	// read the original name before any change, for the output path
	info, err := mi.UnmarshalInfo()
	if err != nil {
		result.Error = fmt.Errorf("could not unmarshal info: %w", err)
		return result, result.Error
	}
	originalMetaInfoName := info.Name

	spec := modifySpec(opts, presetOpts)
	wasModified, err := applyMetadata(mi, spec)
	if err != nil {
		result.Error = err
		return result, result.Error
	}
	// a kept creation date becomes now when anything else changes, or when
	// the preset asks for a date and the torrent has none
	presetWantsDate := presetOpts != nil && presetOpts.NoDate != nil && !*presetOpts.NoDate
	if spec.CreationDate.action == keepField && (wasModified || presetWantsDate && mi.CreationDate == 0) {
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

	// determine output directory: command-line flag takes precedence over preset
	outputDir := opts.OutputDir
	if outputDir == "" && presetOpts != nil && presetOpts.OutputDir != "" {
		outputDir = presetOpts.OutputDir
	}

	// generate output path using the preset generating helper
	var trackerForOutput string
	if len(opts.TrackerURLs) > 0 {
		trackerForOutput = opts.TrackerURLs[0]
	} else {
		trackerForOutput = ""
	}
	outPath := preset.GenerateOutputPath(basePath, outputDir, opts.PresetName, opts.OutputPattern, trackerForOutput, metaInfoName, opts.SkipPrefix)
	result.OutputPath = outPath

	// ensure output directory exists if specified
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			result.Error = fmt.Errorf("could not create output directory: %w", err)
			return result, result.Error
		}
	}

	// save modified torrent file
	f, err := os.Create(outPath)
	if err != nil {
		result.Error = fmt.Errorf("could not create output file: %w", err)
		return result, result.Error
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		result.Error = fmt.Errorf("could not write output file: %w", err)
		return result, result.Error
	}

	result.WasModified = true
	return result, nil
}

// modifySpec resolves the overrides in opts and the preset p into one
// metadataSpec. The order is override, then preset, then keep. Only an
// override can clear a setting.
func modifySpec(opts ModifyOptions, p *preset.Options) metadataSpec {
	if p == nil {
		p = &preset.Options{}
	}
	var spec metadataSpec

	switch {
	case len(opts.TrackerURLs) > 0:
		spec.Trackers = setTo(opts.TrackerURLs)
	case len(p.Trackers) > 0:
		spec.Trackers = setTo(p.Trackers)
	}

	switch {
	case len(opts.WebSeeds) > 0:
		spec.WebSeeds = opts.WebSeeds
	case len(p.WebSeeds) > 0:
		spec.WebSeeds = p.WebSeeds
	}

	spec.Comment = stringSetting(opts.Comment, opts.CommentSet, p.Comment)
	spec.Source = stringSetting(opts.Source, opts.SourceSet, p.Source)

	if opts.Name != "" {
		spec.Name = setTo(opts.Name)
	}

	switch {
	case opts.RemovePrivate:
		spec.Private = cleared[bool]()
	case opts.IsPrivate != nil:
		spec.Private = setTo(*opts.IsPrivate)
	case p.Private != nil:
		spec.Private = setTo(*p.Private)
	}

	switch {
	case opts.NoCreator, p.NoCreator != nil && *p.NoCreator:
		spec.CreatedBy = cleared[string]()
	case p.NoCreator != nil:
		spec.CreatedBy = setTo(createdBy(opts.Version))
	}

	if opts.NoDate || p.NoDate != nil && *p.NoDate {
		spec.CreationDate = cleared[int64]()
	}

	// false does not remove entropy: no setting asks for that yet
	entropy := opts.Entropy
	if entropy == nil {
		entropy = p.Entropy
	}
	if entropy != nil && *entropy {
		spec.Entropy = setField
	}

	return spec
}

// stringSetting resolves a string setting. An explicit override sets the
// value, or clears it when the value is empty. A preset cannot clear.
func stringSetting(override string, overrideSet bool, presetValue string) field[string] {
	switch {
	case overrideSet && override == "":
		return cleared[string]()
	case override != "":
		return setTo(override)
	case presetValue != "":
		return setTo(presetValue)
	}
	return field[string]{}
}

// ProcessTorrents modifies multiple torrent files according to the given options.
// It processes each torrent file and returns the results for all operations.
// This function provides parallel processing for better performance with multiple files.
func ProcessTorrents(paths []string, opts ModifyOptions) ([]*Result, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no torrent files specified")
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
