// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"cmp"
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

var errPieceLengthAndTargetCount = errors.New("cannot set both piece length and target piece count; use one or the other")

func ResolveCreateOptions(o CreateOverrides, p *CreateSettings) (CreateOptions, error) {
	if p == nil {
		p = &CreateSettings{}
	}

	if o.PieceLengthExp != nil && o.TargetPieceCount != nil {
		return CreateOptions{}, errPieceLengthAndTargetCount
	}
	if p.PieceLengthExp != nil && p.TargetPieceCount != nil {
		return CreateOptions{}, errPieceLengthAndTargetCount
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

		IsPrivate:               pick(o.IsPrivate, p.IsPrivate, true),
		NoDate:                  pick(o.NoDate, p.NoDate, false),
		NoCreator:               pick(o.NoCreator, p.NoCreator, false),
		Entropy:                 pick(o.Entropy, p.Entropy, false),
		SkipPrefix:              pick(o.SkipPrefix, p.SkipPrefix, false),
		FailOnSeasonPackWarning: pick(o.FailOnSeasonPackWarning, p.FailOnSeasonPackWarning, false),
		Comment:                 pick(o.Comment, p.Comment, ""),
		OutputDir:               pick(o.OutputDir, p.OutputDir, ""),
		Workers:                 pick(o.Workers, p.Workers, 0),
		MaxPieceLength:          cmp.Or(o.MaxPieceLength, p.MaxPieceLength),
		TrackerURLs:             o.TrackerURLs,
		WebSeeds:                o.WebSeeds,
		ExcludePatterns:         slices.Concat(p.ExcludePatterns, o.ExcludePatterns),
		IncludePatterns:         slices.Concat(p.IncludePatterns, o.IncludePatterns),
	}
	if opts.TrackerURLs == nil {
		opts.TrackerURLs = slices.Clone(p.TrackerURLs)
	}
	if opts.WebSeeds == nil {
		opts.WebSeeds = slices.Clone(p.WebSeeds)
	}

	// An override of either piece length or target piece count hides both preset values.
	if o.PieceLengthExp != nil || o.TargetPieceCount != nil {
		opts.PieceLengthExp = o.PieceLengthExp
		opts.TargetPieceCount = o.TargetPieceCount
	} else {
		opts.PieceLengthExp = p.PieceLengthExp
		opts.TargetPieceCount = p.TargetPieceCount
	}

	switch {
	case o.Source != nil:
		opts.Source = *o.Source
	case p.Source != nil:
		opts.Source = *p.Source
	case len(opts.TrackerURLs) > 0:
		rules, _ := trackers.Lookup(opts.TrackerURLs[0])
		opts.Source = rules.DefaultSource
	}

	return opts, nil
}

// pick returns the override if set, then the preset value if set, then def.
func pick[T any](override, presetValue *T, def T) T {
	if override != nil {
		return *override
	}
	if presetValue != nil {
		return *presetValue
	}
	return def
}
