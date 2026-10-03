// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"cmp"
	"errors"
	"slices"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/trackers"
)

// CreateSettings are the create settings that a caller gives as overrides.
// The CLI builds this type, and the GUI create request and the batch job embed it.
// A nil pointer or a nil slice means "not set". Then the preset or the default applies.
// A batch job cannot set OutputDir or Workers.
type CreateSettings struct {
	PieceLengthExp          *uint    `json:"pieceLengthExp" yaml:"piece_length"`
	MaxPieceLength          *uint    `json:"maxPieceLength" yaml:"max_piece_length"`
	TargetPieceCount        *uint    `json:"targetPieceCount" yaml:"target_piece_count"`
	Comment                 *string  `json:"comment" yaml:"comment"`
	Source                  *string  `json:"source" yaml:"source"`
	OutputDir               *string  `json:"outputDir" yaml:"-"`
	Workers                 *int     `json:"workers" yaml:"-"`
	IsPrivate               *bool    `json:"isPrivate" yaml:"private"`
	NoDate                  *bool    `json:"noDate" yaml:"no_date"`
	NoCreator               *bool    `json:"noCreator" yaml:"no_creator"`
	Entropy                 *bool    `json:"entropy" yaml:"entropy"`
	SkipPrefix              *bool    `json:"skipPrefix" yaml:"skip_prefix"`
	FailOnSeasonPackWarning *bool    `json:"failOnSeasonWarning" yaml:"fail_on_season_warning"`
	TrackerURLs             []string `json:"trackerUrls" yaml:"trackers"`
	WebSeeds                []string `json:"webSeeds" yaml:"webseeds"`
	ExcludePatterns         []string `json:"excludePatterns" yaml:"exclude_patterns"`
	IncludePatterns         []string `json:"includePatterns" yaml:"include_patterns"`
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

// ResolveCreateOptions resolves the overrides, the preset, and the tracker defaults
// into the final create options. An override wins over the preset, and the preset
// wins over the tracker default. The preset can be nil.
func ResolveCreateOptions(o CreateOverrides, p *preset.Options) (CreateOptions, error) {
	if p == nil {
		p = &preset.Options{}
	}

	if o.PieceLengthExp != nil && o.TargetPieceCount != nil {
		return CreateOptions{}, errPieceLengthAndTargetCount
	}
	if p.PieceLength != 0 && p.TargetPieceCount != 0 {
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

		IsPrivate:               pick(o.IsPrivate, p.Private, true),
		NoDate:                  pick(o.NoDate, p.NoDate, false),
		NoCreator:               pick(o.NoCreator, p.NoCreator, false),
		Entropy:                 pick(o.Entropy, p.Entropy, false),
		SkipPrefix:              pick(o.SkipPrefix, p.SkipPrefix, false),
		FailOnSeasonPackWarning: pick(o.FailOnSeasonPackWarning, p.FailOnSeasonWarning, false),
		Comment:                 pick(o.Comment, nonZero(p.Comment), ""),
		OutputDir:               pick(o.OutputDir, nonZero(p.OutputDir), ""),
		Workers:                 pick(o.Workers, nonZero(p.Workers), 0),
		MaxPieceLength:          cmp.Or(o.MaxPieceLength, nonZero(p.MaxPieceLength)),
		TrackerURLs:             o.TrackerURLs,
		WebSeeds:                o.WebSeeds,
		ExcludePatterns:         slices.Concat(p.ExcludePatterns, o.ExcludePatterns),
		IncludePatterns:         slices.Concat(p.IncludePatterns, o.IncludePatterns),
	}
	if opts.TrackerURLs == nil {
		opts.TrackerURLs = slices.Clone(p.Trackers)
	}
	if opts.WebSeeds == nil {
		opts.WebSeeds = slices.Clone(p.WebSeeds)
	}

	// An override of either piece length or target piece count hides both preset values.
	if o.PieceLengthExp != nil || o.TargetPieceCount != nil {
		opts.PieceLengthExp = o.PieceLengthExp
		opts.TargetPieceCount = o.TargetPieceCount
	} else {
		opts.PieceLengthExp = nonZero(p.PieceLength)
		opts.TargetPieceCount = nonZero(p.TargetPieceCount)
	}

	switch {
	case o.Source != nil:
		opts.Source = *o.Source
	case p.Source != "":
		opts.Source = p.Source
	case len(opts.TrackerURLs) > 0:
		opts.Source, _ = trackers.GetTrackerDefaultSource(opts.TrackerURLs[0])
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

// nonZero returns nil for the zero value. A preset uses the zero value for "not set".
func nonZero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
