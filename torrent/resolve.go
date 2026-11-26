// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"errors"
	"slices"

	"github.com/autobrr/mkbrr/internal/trackers"
)

// A batch job cannot set OutputDir or Workers.
// The flag tag names the CLI flag, and the preset tag names the preset key.
// TestSettingsParity in cmd checks both tags.
type CreateSettings struct {
	PieceLengthExp          *uint    `json:"pieceLengthExp" yaml:"piece_length" flag:"piece-length" preset:"piece_length"`
	MaxPieceLength          *uint    `json:"maxPieceLength" yaml:"max_piece_length" flag:"max-piece-length" preset:"max_piece_length"`
	TargetPieceCount        *uint    `json:"targetPieceCount" yaml:"target_piece_count" flag:"target-piece-count" preset:"target_piece_count"`
	Comment                 *string  `json:"comment" yaml:"comment" flag:"comment" preset:"comment"`
	Source                  *string  `json:"source" yaml:"source" flag:"source" preset:"source"`
	OutputDir               *string  `json:"outputDir" yaml:"-" flag:"output-dir" preset:"output_dir"`
	Workers                 *int     `json:"workers" yaml:"-" flag:"workers" preset:"workers"`
	IsPrivate               *bool    `json:"isPrivate" yaml:"private" flag:"private" preset:"private"`
	NoDate                  *bool    `json:"noDate" yaml:"no_date" flag:"no-date" preset:"no_date"`
	NoCreator               *bool    `json:"noCreator" yaml:"no_creator" flag:"no-creator" preset:"no_creator"`
	Entropy                 *bool    `json:"entropy" yaml:"entropy" flag:"entropy" preset:"entropy"`
	SkipPrefix              *bool    `json:"skipPrefix" yaml:"skip_prefix" flag:"skip-prefix" preset:"skip_prefix"`
	FailOnSeasonPackWarning *bool    `json:"failOnSeasonWarning" yaml:"fail_on_season_warning" flag:"fail-on-season-warning" preset:"fail_on_season_warning"`
	TrackerURLs             []string `json:"trackerUrls" yaml:"trackers" flag:"tracker" preset:"trackers"`
	WebSeeds                []string `json:"webSeeds" yaml:"webseeds" flag:"web-seed" preset:"webseeds"`
	ExcludePatterns         []string `json:"excludePatterns" yaml:"exclude_patterns" flag:"exclude" preset:"exclude_patterns"`
	IncludePatterns         []string `json:"includePatterns" yaml:"include_patterns" flag:"include" preset:"include_patterns"`
}

// CreateOverrides holds the create settings that the caller gave explicitly,
// and the runtime fields. The runtime fields are not settings.
// ResolveCreateOptions copies them without change.
type CreateOverrides struct {
	CreateSettings

	Path             string
	Name             string
	OutputPath       string
	Version          string
	Verbose          bool
	Quiet            bool
	InfoOnly         bool
	ProgressCallback ProgressCallback
}

type Origin string

const (
	OriginOverride       Origin = "override"
	OriginPreset         Origin = "preset"
	OriginTrackerDefault Origin = "tracker"
	OriginDefault        Origin = "default"
)

type CreateOrigins struct {
	PieceLengthExp          Origin `json:"pieceLengthExp"`
	MaxPieceLength          Origin `json:"maxPieceLength"`
	TargetPieceCount        Origin `json:"targetPieceCount"`
	Comment                 Origin `json:"comment"`
	Source                  Origin `json:"source"`
	OutputDir               Origin `json:"outputDir"`
	Workers                 Origin `json:"workers"`
	IsPrivate               Origin `json:"isPrivate"`
	NoDate                  Origin `json:"noDate"`
	NoCreator               Origin `json:"noCreator"`
	Entropy                 Origin `json:"entropy"`
	SkipPrefix              Origin `json:"skipPrefix"`
	FailOnSeasonPackWarning Origin `json:"failOnSeasonWarning"`
	TrackerURLs             Origin `json:"trackerUrls"`
	WebSeeds                Origin `json:"webSeeds"`
	ExcludePatterns         Origin `json:"excludePatterns"`
	IncludePatterns         Origin `json:"includePatterns"`
}

var errPieceLengthAndTargetCount = errors.New("cannot set both piece length and target piece count; use one or the other")

func ResolveCreateOptions(o CreateOverrides, p *CreateSettings) (CreateOptions, error) {
	opts, _, err := ResolveCreate(o, p)
	return opts, err
}

func ResolveCreate(o CreateOverrides, p *CreateSettings) (CreateOptions, CreateOrigins, error) {
	if p == nil {
		p = &CreateSettings{}
	}

	if o.PieceLengthExp != nil && o.TargetPieceCount != nil {
		return CreateOptions{}, CreateOrigins{}, errPieceLengthAndTargetCount
	}
	if p.PieceLengthExp != nil && p.TargetPieceCount != nil {
		return CreateOptions{}, CreateOrigins{}, errPieceLengthAndTargetCount
	}

	opts := CreateOptions{
		Path:             o.Path,
		Name:             o.Name,
		OutputPath:       o.OutputPath,
		Version:          o.Version,
		Verbose:          o.Verbose,
		Quiet:            o.Quiet,
		InfoOnly:         o.InfoOnly,
		ProgressCallback: o.ProgressCallback,
	}
	var origins CreateOrigins

	opts.IsPrivate, origins.IsPrivate = pick(o.IsPrivate, p.IsPrivate, true)
	opts.NoDate, origins.NoDate = pick(o.NoDate, p.NoDate, false)
	opts.NoCreator, origins.NoCreator = pick(o.NoCreator, p.NoCreator, false)
	opts.Entropy, origins.Entropy = pick(o.Entropy, p.Entropy, false)
	opts.SkipPrefix, origins.SkipPrefix = pick(o.SkipPrefix, p.SkipPrefix, false)
	opts.FailOnSeasonPackWarning, origins.FailOnSeasonPackWarning = pick(o.FailOnSeasonPackWarning, p.FailOnSeasonPackWarning, false)
	opts.Comment, origins.Comment = pick(o.Comment, p.Comment, "")
	opts.OutputDir, origins.OutputDir = pick(o.OutputDir, p.OutputDir, "")
	opts.Workers, origins.Workers = pick(o.Workers, p.Workers, 0)
	opts.MaxPieceLength, origins.MaxPieceLength = pickPointer(o.MaxPieceLength, p.MaxPieceLength)
	opts.TrackerURLs, origins.TrackerURLs = pickList(o.TrackerURLs, p.TrackerURLs)
	opts.WebSeeds, origins.WebSeeds = pickList(o.WebSeeds, p.WebSeeds)
	opts.ExcludePatterns, origins.ExcludePatterns = addPatterns(o.ExcludePatterns, p.ExcludePatterns)
	opts.IncludePatterns, origins.IncludePatterns = addPatterns(o.IncludePatterns, p.IncludePatterns)

	// An override of either piece length or target piece count hides both preset values.
	if o.PieceLengthExp != nil || o.TargetPieceCount != nil {
		opts.PieceLengthExp, origins.PieceLengthExp = pickPointer(o.PieceLengthExp, nil)
		opts.TargetPieceCount, origins.TargetPieceCount = pickPointer(o.TargetPieceCount, nil)
	} else {
		opts.PieceLengthExp, origins.PieceLengthExp = pickPointer(nil, p.PieceLengthExp)
		opts.TargetPieceCount, origins.TargetPieceCount = pickPointer(nil, p.TargetPieceCount)
	}

	opts.Source, origins.Source = pick(o.Source, p.Source, "")
	if origins.Source == OriginDefault && len(opts.TrackerURLs) > 0 {
		if rules, _ := trackers.Lookup(opts.TrackerURLs[0]); rules.DefaultSource != "" {
			opts.Source, origins.Source = rules.DefaultSource, OriginTrackerDefault
		}
	}

	return opts, origins, nil
}

func pick[T any](override, presetValue *T, def T) (T, Origin) {
	if override != nil {
		return *override, OriginOverride
	}
	if presetValue != nil {
		return *presetValue, OriginPreset
	}
	return def, OriginDefault
}

func pickPointer[T any](override, presetValue *T) (*T, Origin) {
	switch {
	case override != nil:
		return override, OriginOverride
	case presetValue != nil:
		return presetValue, OriginPreset
	default:
		return nil, OriginDefault
	}
}

func pickList(override, presetValue []string) ([]string, Origin) {
	switch {
	case override != nil:
		return override, OriginOverride
	case presetValue != nil:
		return slices.Clone(presetValue), OriginPreset
	default:
		return nil, OriginDefault
	}
}

func addPatterns(override, presetValue []string) ([]string, Origin) {
	patterns := slices.Concat(presetValue, override)
	switch {
	case len(presetValue) > 0:
		return patterns, OriginPreset
	case len(override) > 0:
		return patterns, OriginOverride
	default:
		return patterns, OriginDefault
	}
}
