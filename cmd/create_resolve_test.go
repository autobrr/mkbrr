// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/autobrr/mkbrr/torrent"
)

const (
	guardContent      = "/data/Show.S01"
	guardAntTracker   = "https://anthelion.me/announce/abc"
	guardOtherTracker = "https://tracker.example.invalid/announce"
)

const guardPresets = `version: 1
default:
  private: false
  no_creator: true
  comment: default comment
  webseeds: [https://seed.example.invalid/default/]
  exclude_patterns: ["*.txt"]
  target_piece_count: 500
  output_dir: /default-out
presets:
  full:
    private: true
    no_date: true
    no_creator: false
    skip_prefix: true
    entropy: true
    fail_on_season_warning: true
    comment: preset comment
    source: PRESET
    output_dir: /preset-out
    trackers: [https://tracker.example.invalid/announce]
    webseeds: [https://seed.example.invalid/preset/]
    exclude_patterns: ["*.nfo"]
    include_patterns: ["*.mkv"]
    piece_length: 20
    max_piece_length: 22
    workers: 3
  inherits: {}
`

const guardPlainPresets = `version: 1
presets:
  ant:
    trackers: [https://anthelion.me/announce/abc]
`

func resolveCreateFlags(t *testing.T, args ...string) torrent.CreateOptions {
	t.Helper()
	t.Cleanup(func() {
		options = createOptions{isPrivate: true}
		createCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	})
	require.NoError(t, createCmd.ParseFlags(args))
	opts, err := buildCreateOptions(createCmd, guardContent, options, "test")
	require.NoError(t, err)
	return opts
}

func TestCreateResolvedOptionsGuard(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Chdir(dir)
	presets := filepath.Join(dir, "guard-presets.yaml")
	require.NoError(t, os.WriteFile(presets, []byte(guardPresets), 0o600))
	plain := filepath.Join(dir, "plain-presets.yaml")
	require.NoError(t, os.WriteFile(plain, []byte(guardPlainPresets), 0o600))

	flagRows := []struct {
		name string
		args []string
		want torrent.CreateOptions
	}{
		{
			name: "no preset and no flags",
			want: torrent.CreateOptions{Path: guardContent, Version: "test", IsPrivate: true},
		},
		{
			name: "flags only",
			args: []string{
				"-t", guardOtherTracker, "-w", "https://seed.example.invalid/flag/", "--private=false",
				"-c", "flag comment", "-l", "18", "-m", "24", "--name", "Named", "-o", "out.torrent",
				"--output-dir", "/flag-out", "-s", "FLAG", "-d", "--no-creator", "-e", "--skip-prefix",
				"--fail-on-season-warning", "--exclude", "*.nfo", "--include", "*.mkv", "--workers", "2", "-v", "-i",
			},
			want: torrent.CreateOptions{
				Path:                    guardContent,
				Name:                    "Named",
				OutputPath:              "out.torrent",
				Version:                 "test",
				Verbose:                 true,
				InfoOnly:                true,
				TrackerURLs:             []string{guardOtherTracker},
				WebSeeds:                []string{"https://seed.example.invalid/flag/"},
				Comment:                 "flag comment",
				Source:                  "FLAG",
				OutputDir:               "/flag-out",
				PieceLengthExp:          new(uint(18)),
				MaxPieceLength:          new(uint(24)),
				ExcludePatterns:         []string{"*.nfo"},
				IncludePatterns:         []string{"*.mkv"},
				Workers:                 2,
				NoDate:                  true,
				NoCreator:               true,
				Entropy:                 true,
				SkipPrefix:              true,
				FailOnSeasonPackWarning: true,
			},
		},
		{
			name: "flags only, tracker default source and target piece count",
			args: []string{"-t", guardAntTracker, "--target-piece-count", "1000", "-q"},
			want: torrent.CreateOptions{
				Path:             guardContent,
				Version:          "test",
				Quiet:            true,
				IsPrivate:        true,
				TrackerURLs:      []string{guardAntTracker},
				Source:           "ANT",
				TargetPieceCount: new(uint(1000)),
			},
		},
		{
			name: "preset only, every key set",
			args: []string{"--preset-file", presets, "-P", "full"},
			want: torrent.CreateOptions{
				Path:                    guardContent,
				Version:                 "test",
				IsPrivate:               true,
				NoDate:                  true,
				SkipPrefix:              true,
				Entropy:                 true,
				FailOnSeasonPackWarning: true,
				Comment:                 "preset comment",
				Source:                  "PRESET",
				OutputDir:               "/preset-out",
				TrackerURLs:             []string{guardOtherTracker},
				WebSeeds:                []string{"https://seed.example.invalid/preset/"},
				ExcludePatterns:         []string{"*.nfo"},
				IncludePatterns:         []string{"*.mkv"},
				PieceLengthExp:          new(uint(20)),
				MaxPieceLength:          new(uint(22)),
				Workers:                 3,
			},
		},
		{
			name: "preset only, values from the default section",
			args: []string{"--preset-file", presets, "-P", "inherits"},
			want: torrent.CreateOptions{
				Path:             guardContent,
				Version:          "test",
				NoCreator:        true,
				Comment:          "default comment",
				OutputDir:        "/default-out",
				WebSeeds:         []string{"https://seed.example.invalid/default/"},
				ExcludePatterns:  []string{"*.txt"},
				TargetPieceCount: new(uint(500)),
			},
		},
		{
			name: "preset only, create defaults and tracker default source",
			args: []string{"--preset-file", plain, "-P", "ant"},
			want: torrent.CreateOptions{
				Path:        guardContent,
				Version:     "test",
				IsPrivate:   true,
				TrackerURLs: []string{guardAntTracker},
				Source:      "ANT",
			},
		},
		{
			name: "preset plus flags",
			args: []string{
				"--preset-file", presets, "-P", "full",
				"--private=false", "--no-date=false", "-s", "", "-c", "", "-t", guardAntTracker,
				"--target-piece-count", "1000", "--exclude", "*.jpg", "--include", "*.mp4", "--workers", "1",
			},
			want: torrent.CreateOptions{
				Path:                    guardContent,
				Version:                 "test",
				SkipPrefix:              true,
				Entropy:                 true,
				FailOnSeasonPackWarning: true,
				OutputDir:               "/preset-out",
				TrackerURLs:             []string{guardAntTracker},
				WebSeeds:                []string{"https://seed.example.invalid/preset/"},
				ExcludePatterns:         []string{"*.nfo", "*.jpg"},
				IncludePatterns:         []string{"*.mkv", "*.mp4"},
				TargetPieceCount:        new(uint(1000)),
				MaxPieceLength:          new(uint(22)),
				Workers:                 1,
			},
		},
		{
			name: "preset from the default section plus flags",
			args: []string{"--preset-file", presets, "-P", "inherits", "-l", "19", "-p", "--output-dir", "/flag-out"},
			want: torrent.CreateOptions{
				Path:            guardContent,
				Version:         "test",
				IsPrivate:       true,
				NoCreator:       true,
				Comment:         "default comment",
				OutputDir:       "/flag-out",
				WebSeeds:        []string{"https://seed.example.invalid/default/"},
				ExcludePatterns: []string{"*.txt"},
				PieceLengthExp:  new(uint(19)),
			},
		},
	}
	for _, tt := range flagRows {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveCreateFlags(t, tt.args...))
		})
	}

	batchRows := []struct {
		name string
		job  string
		want torrent.CreateOptions
	}{
		{
			name: "batch job, every key set",
			job: `path: /data/Batch
output: batch.torrent
trackers: [https://tracker.example.invalid/announce]
webseeds: [https://seed.example.invalid/batch/]
exclude_patterns: ["*.nfo"]
include_patterns: ["*.mkv"]
comment: batch comment
source: BATCH
private: false
piece_length: 0
max_piece_length: 23
target_piece_count: 800
no_date: true
no_creator: true
skip_prefix: true
entropy: true
fail_on_season_warning: true
`,
			want: torrent.CreateOptions{
				Path:                    "/data/Batch",
				Version:                 "test",
				TrackerURLs:             []string{guardOtherTracker},
				WebSeeds:                []string{"https://seed.example.invalid/batch/"},
				ExcludePatterns:         []string{"*.nfo"},
				IncludePatterns:         []string{"*.mkv"},
				Comment:                 "batch comment",
				Source:                  "BATCH",
				MaxPieceLength:          new(uint(23)),
				TargetPieceCount:        new(uint(800)),
				NoDate:                  true,
				NoCreator:               true,
				SkipPrefix:              true,
				Entropy:                 true,
				FailOnSeasonPackWarning: true,
			},
		},
		{
			name: "batch job, create defaults and tracker default source",
			job:  "path: /data/Batch\ntrackers: [https://anthelion.me/announce/abc]\n",
			want: torrent.CreateOptions{
				Path:        "/data/Batch",
				Version:     "test",
				IsPrivate:   true,
				TrackerURLs: []string{guardAntTracker},
				Source:      "ANT",
			},
		},
	}
	for _, tt := range batchRows {
		t.Run(tt.name, func(t *testing.T) {
			var job torrent.BatchJob
			require.NoError(t, yaml.Unmarshal([]byte(tt.job), &job))
			got, err := job.ToCreateOptions(false, false, false, "test")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
