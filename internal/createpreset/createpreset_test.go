// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package createpreset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/mkbrr/torrent"
)

func writePresets(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presets.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	return path
}

func TestLoadEveryKey(t *testing.T) {
	path := writePresets(t, `version: 1
default:
  comment: default comment
  max_piece_length: 23
presets:
  all:
    private: false
    no_date: true
    no_creator: true
    skip_prefix: true
    entropy: true
    fail_on_season_warning: true
    source: SRC
    output_dir: /out
    trackers: [https://tracker.example.invalid/announce]
    webseeds: [https://seed.example.invalid/]
    exclude_patterns: ["*.nfo"]
    include_patterns: ["*.mkv"]
    target_piece_count: 1000
    workers: 4
`)

	got, err := Load(path, "all")
	require.NoError(t, err)
	assert.Equal(t, &torrent.CreateSettings{
		IsPrivate:               new(false),
		NoDate:                  new(true),
		NoCreator:               new(true),
		SkipPrefix:              new(true),
		Entropy:                 new(true),
		FailOnSeasonPackWarning: new(true),
		Comment:                 new("default comment"),
		Source:                  new("SRC"),
		OutputDir:               new("/out"),
		TrackerURLs:             []string{"https://tracker.example.invalid/announce"},
		WebSeeds:                []string{"https://seed.example.invalid/"},
		ExcludePatterns:         []string{"*.nfo"},
		IncludePatterns:         []string{"*.mkv"},
		MaxPieceLength:          new(uint(23)),
		TargetPieceCount:        new(uint(1000)),
		Workers:                 new(4),
	}, got)
}

func TestLoadZeroIsNotSet(t *testing.T) {
	path := writePresets(t, `version: 1
presets:
  zero:
    comment: ""
    source: ""
    output_dir: ""
    piece_length: 0
    max_piece_length: 0
    target_piece_count: 0
    workers: 0
`)

	got, err := Load(path, "zero")
	require.NoError(t, err)
	assert.Equal(t, &torrent.CreateSettings{}, got)
}

func TestLoadUnknownPreset(t *testing.T) {
	path := writePresets(t, "version: 1\npresets:\n  p: {}\n")

	_, err := Load(path, "missing")
	require.EqualError(t, err, `could not get preset: preset "missing" not found`)
}
