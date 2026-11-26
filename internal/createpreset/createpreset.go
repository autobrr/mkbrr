// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package createpreset

import (
	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/torrent"
)

func Load(presetFile, name string) (*torrent.CreateSettings, error) {
	values, err := preset.LoadPresetValues(presetFile, name)
	if err != nil {
		return nil, err
	}
	return new(settings(*values)), nil
}

func settings(o preset.Options) torrent.CreateSettings {
	return torrent.CreateSettings{
		PieceLengthExp:          nonZero(o.PieceLength),
		MaxPieceLength:          nonZero(o.MaxPieceLength),
		TargetPieceCount:        nonZero(o.TargetPieceCount),
		Comment:                 nonZero(o.Comment),
		Source:                  nonZero(o.Source),
		OutputDir:               nonZero(o.OutputDir),
		Workers:                 nonZero(o.Workers),
		IsPrivate:               o.Private,
		NoDate:                  o.NoDate,
		NoCreator:               o.NoCreator,
		Entropy:                 o.Entropy,
		SkipPrefix:              o.SkipPrefix,
		FailOnSeasonPackWarning: o.FailOnSeasonWarning,
		TrackerURLs:             o.Trackers,
		WebSeeds:                o.WebSeeds,
		ExcludePatterns:         o.ExcludePatterns,
		IncludePatterns:         o.IncludePatterns,
	}
}

func nonZero[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
