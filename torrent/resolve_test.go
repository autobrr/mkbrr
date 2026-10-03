// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/autobrr/mkbrr/internal/preset"
)

func TestResolveCreateOptions(t *testing.T) {
	const antTracker = "https://anthelion.me/announce/abc"
	const otherTracker = "https://tracker.example.invalid/announce"

	tests := []struct {
		name      string
		overrides CreateOverrides
		preset    *preset.Options
		want      CreateOptions
		wantErr   bool
	}{
		{
			name: "no override and no preset uses defaults",
			want: CreateOptions{IsPrivate: true},
		},
		{
			name: "runtime fields are copied",
			overrides: CreateOverrides{
				Path:       "/data/x",
				Name:       "n",
				OutputPath: "out.torrent",
				Version:    "1.2.3",
				Verbose:    true,
				Quiet:      true,
				InfoOnly:   true,
			},
			want: CreateOptions{
				Path:       "/data/x",
				Name:       "n",
				OutputPath: "out.torrent",
				Version:    "1.2.3",
				Verbose:    true,
				Quiet:      true,
				InfoOnly:   true,
				IsPrivate:  true,
			},
		},
		{
			name: "preset fills settings that no override sets",
			preset: &preset.Options{
				Private:             new(false),
				NoDate:              new(true),
				NoCreator:           new(true),
				SkipPrefix:          new(true),
				Entropy:             new(true),
				FailOnSeasonWarning: new(true),
				Comment:             "c",
				Source:              "s",
				OutputDir:           "/out",
				Trackers:            []string{otherTracker},
				WebSeeds:            []string{"https://seed.example.invalid"},
				ExcludePatterns:     []string{"*.nfo"},
				IncludePatterns:     []string{"*.mkv"},
				PieceLength:         20,
				MaxPieceLength:      22,
				Workers:             3,
			},
			want: CreateOptions{
				IsPrivate:               false,
				NoDate:                  true,
				NoCreator:               true,
				SkipPrefix:              true,
				Entropy:                 true,
				FailOnSeasonPackWarning: true,
				Comment:                 "c",
				Source:                  "s",
				OutputDir:               "/out",
				TrackerURLs:             []string{otherTracker},
				WebSeeds:                []string{"https://seed.example.invalid"},
				ExcludePatterns:         []string{"*.nfo"},
				IncludePatterns:         []string{"*.mkv"},
				PieceLengthExp:          new(uint(20)),
				MaxPieceLength:          new(uint(22)),
				Workers:                 3,
			},
		},
		{
			name: "overrides win over preset, including false and empty values",
			overrides: CreateOverrides{
				IsPrivate:               new(true),
				NoDate:                  new(false),
				NoCreator:               new(false),
				SkipPrefix:              new(false),
				Entropy:                 new(false),
				FailOnSeasonPackWarning: new(false),
				Comment:                 new(""),
				Source:                  new(""),
				OutputDir:               new("/flag"),
				TrackerURLs:             []string{},
				WebSeeds:                []string{},
				PieceLengthExp:          new(uint(18)),
				MaxPieceLength:          new(uint(19)),
				Workers:                 new(1),
			},
			preset: &preset.Options{
				Private:             new(false),
				NoDate:              new(true),
				NoCreator:           new(true),
				SkipPrefix:          new(true),
				Entropy:             new(true),
				FailOnSeasonWarning: new(true),
				Comment:             "c",
				Source:              "s",
				OutputDir:           "/out",
				Trackers:            []string{antTracker},
				WebSeeds:            []string{"https://seed.example.invalid"},
				PieceLength:         20,
				MaxPieceLength:      22,
				Workers:             3,
			},
			want: CreateOptions{
				IsPrivate:      true,
				OutputDir:      "/flag",
				TrackerURLs:    []string{},
				WebSeeds:       []string{},
				PieceLengthExp: new(uint(18)),
				MaxPieceLength: new(uint(19)),
				Workers:        1,
			},
		},
		{
			name: "override patterns are added after preset patterns",
			overrides: CreateOverrides{
				ExcludePatterns: []string{"*.jpg"},
				IncludePatterns: []string{"*.mp4"},
			},
			preset: &preset.Options{
				ExcludePatterns: []string{"*.nfo"},
				IncludePatterns: []string{"*.mkv"},
			},
			want: CreateOptions{
				IsPrivate:       true,
				ExcludePatterns: []string{"*.nfo", "*.jpg"},
				IncludePatterns: []string{"*.mkv", "*.mp4"},
			},
		},
		{
			name:      "override target piece count suppresses preset piece length",
			overrides: CreateOverrides{TargetPieceCount: new(uint(1000))},
			preset:    &preset.Options{PieceLength: 20},
			want:      CreateOptions{IsPrivate: true, TargetPieceCount: new(uint(1000))},
		},
		{
			name:      "override piece length suppresses preset target piece count",
			overrides: CreateOverrides{PieceLengthExp: new(uint(20))},
			preset:    &preset.Options{TargetPieceCount: 1000},
			want:      CreateOptions{IsPrivate: true, PieceLengthExp: new(uint(20))},
		},
		{
			name:   "preset target piece count applies",
			preset: &preset.Options{TargetPieceCount: 1000},
			want:   CreateOptions{IsPrivate: true, TargetPieceCount: new(uint(1000))},
		},
		{
			name:      "override with both piece length and target piece count is an error",
			overrides: CreateOverrides{PieceLengthExp: new(uint(20)), TargetPieceCount: new(uint(1000))},
			wantErr:   true,
		},
		{
			name:    "preset with both piece length and target piece count is an error",
			preset:  &preset.Options{PieceLength: 20, TargetPieceCount: 1000},
			wantErr: true,
		},
		{
			name:      "tracker default source applies when no override and no preset sets source",
			overrides: CreateOverrides{TrackerURLs: []string{antTracker}},
			want:      CreateOptions{IsPrivate: true, TrackerURLs: []string{antTracker}, Source: "ANT"},
		},
		{
			name:   "tracker default source uses preset trackers",
			preset: &preset.Options{Trackers: []string{antTracker}},
			want:   CreateOptions{IsPrivate: true, TrackerURLs: []string{antTracker}, Source: "ANT"},
		},
		{
			name:      "preset source wins over tracker default",
			overrides: CreateOverrides{TrackerURLs: []string{antTracker}},
			preset:    &preset.Options{Source: "s"},
			want:      CreateOptions{IsPrivate: true, TrackerURLs: []string{antTracker}, Source: "s"},
		},
		{
			name:      "empty source override suppresses tracker default",
			overrides: CreateOverrides{TrackerURLs: []string{antTracker}, Source: new("")},
			want:      CreateOptions{IsPrivate: true, TrackerURLs: []string{antTracker}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveCreateOptions(tt.overrides, tt.preset)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestResolveCreateOptionsDoesNotAliasPresetPatterns(t *testing.T) {
	p := &preset.Options{ExcludePatterns: make([]string, 1, 4)}
	p.ExcludePatterns[0] = "*.nfo"

	got, err := ResolveCreateOptions(CreateOverrides{ExcludePatterns: []string{"*.jpg"}}, p)
	if err != nil {
		t.Fatal(err)
	}
	got.ExcludePatterns[0] = "changed"
	if p.ExcludePatterns[0] != "*.nfo" {
		t.Errorf("resolver result aliases preset patterns")
	}
}

func TestBatchJobToCreateOptions(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantPrivate bool
		wantSource  string
	}{
		{"defaults to private and tracker default source", "trackers: [https://anthelion.me/announce/x]", true, "ANT"},
		{"empty source suppresses tracker default", "trackers: [https://anthelion.me/announce/x]\nsource: \"\"", true, ""},
		{"private false is kept", "private: false", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var job BatchJob
			if err := yaml.Unmarshal([]byte(tt.yaml), &job); err != nil {
				t.Fatal(err)
			}
			got, err := job.ToCreateOptions(false, false, false, "v")
			if err != nil {
				t.Fatal(err)
			}
			if got.IsPrivate != tt.wantPrivate || got.Source != tt.wantSource {
				t.Errorf("got private=%v source=%q, want private=%v source=%q", got.IsPrivate, got.Source, tt.wantPrivate, tt.wantSource)
			}
		})
	}
}

func TestBatchJobYAMLKeys(t *testing.T) {
	const doc = `
path: /data/x
output: out.torrent
trackers: [https://t.example.invalid/announce]
webseeds: [https://w.example.invalid/]
exclude_patterns: ["*.nfo"]
include_patterns: ["*.mkv"]
comment: c
source: s
private: false
piece_length: 20
max_piece_length: 22
target_piece_count: 1000
no_date: true
no_creator: true
skip_prefix: true
entropy: true
fail_on_season_warning: true
`
	var job BatchJob
	if err := yaml.Unmarshal([]byte(doc), &job); err != nil {
		t.Fatal(err)
	}
	want := BatchJob{
		Path:                    "/data/x",
		Output:                  "out.torrent",
		TrackerURLs:             []string{"https://t.example.invalid/announce"},
		WebSeeds:                []string{"https://w.example.invalid/"},
		ExcludePatterns:         []string{"*.nfo"},
		IncludePatterns:         []string{"*.mkv"},
		Comment:                 new("c"),
		Source:                  new("s"),
		IsPrivate:               new(false),
		PieceLengthExp:          new(uint(20)),
		MaxPieceLength:          new(uint(22)),
		TargetPieceCount:        new(uint(1000)),
		NoDate:                  new(true),
		NoCreator:               new(true),
		SkipPrefix:              new(true),
		Entropy:                 new(true),
		FailOnSeasonPackWarning: new(true),
	}
	if !reflect.DeepEqual(job, want) {
		t.Errorf("got  %+v\nwant %+v", job, want)
	}
}
