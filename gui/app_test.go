// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/torrent"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPlanCreateMatchesCreateTorrent sends the same request to the plan method
// and to the create method. The planned piece length must be the piece length
// of the written torrent.
func TestPlanCreateMatchesCreateTorrent(t *testing.T) {
	const mainSize = 3 << 20

	tests := []struct {
		name string
		// setup returns the request for a content folder in dir.
		setup       func(t *testing.T, dir string) CreateRequest
		wantExp     uint
		wantContent int64
	}{
		{
			// Create applies only the rules of the first tracker, so the
			// 64 KiB range of the second tracker does not apply.
			name: "two trackers, only the second has rules",
			setup: func(t *testing.T, dir string) CreateRequest {
				writeFile(t, filepath.Join(dir, "content", "main.mkv"), mainSize)
				return CreateRequest{
					Path: filepath.Join(dir, "content"),
					TrackerURLs: []string{
						"https://unknown.tracker.test/announce",
						"https://passthepopcorn.me/announce?passkey=123",
					},
				}
			},
			wantExp:     15,
			wantContent: mainSize,
		},
		{
			name: "excluded sample folder",
			setup: func(t *testing.T, dir string) CreateRequest {
				writeFile(t, filepath.Join(dir, "content", "main.mkv"), mainSize)
				writeFile(t, filepath.Join(dir, "content", "Sample", "sample.mkv"), 1<<20)
				return CreateRequest{
					Path:            filepath.Join(dir, "content"),
					ExcludePatterns: []string{"*sample*"},
				}
			},
			wantExp:     15,
			wantContent: mainSize,
		},
		{
			// The tracker caps pieces at 8 MiB. One piece would need 32 MiB.
			name: "target piece count, tracker cap",
			setup: func(t *testing.T, dir string) CreateRequest {
				writeFile(t, filepath.Join(dir, "content", "main.mkv"), 20<<20)
				return CreateRequest{
					Path: filepath.Join(dir, "content"),
					CreateSettings: torrent.CreateSettings{
						TrackerURLs:      []string{"https://morethantv.me/announce"},
						TargetPieceCount: new(uint(1)),
					},
				}
			},
			wantExp:     23,
			wantContent: 20 << 20,
		},
		{
			name: "target piece count, max piece length below the tracker cap",
			setup: func(t *testing.T, dir string) CreateRequest {
				writeFile(t, filepath.Join(dir, "content", "main.mkv"), 20<<20)
				return CreateRequest{
					Path: filepath.Join(dir, "content"),
					CreateSettings: torrent.CreateSettings{
						TrackerURLs:      []string{"https://morethantv.me/announce"},
						TargetPieceCount: new(uint(1)),
						MaxPieceLength:   new(uint(22)),
					},
				}
			},
			wantExp:     22,
			wantContent: 20 << 20,
		},
		{
			name: "preset piece length",
			setup: func(t *testing.T, dir string) CreateRequest {
				writeFile(t, filepath.Join(dir, "content", "main.mkv"), mainSize)
				presetFile := filepath.Join(dir, "presets.yaml")
				presets := "version: 1\npresets:\n  big:\n    piece_length: 18\n"
				if err := os.WriteFile(presetFile, []byte(presets), 0o644); err != nil {
					t.Fatal(err)
				}
				return CreateRequest{
					Path:       filepath.Join(dir, "content"),
					PresetName: "big",
					PresetFile: presetFile,
				}
			},
			wantExp:     18,
			wantContent: mainSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			req := tt.setup(t, dir)
			app := NewApp("test")

			plan, err := app.PlanCreate(req)
			if err != nil {
				t.Fatalf("PlanCreate: %v", err)
			}
			if plan.PieceLengthExponent != tt.wantExp {
				t.Errorf("planned piece length exponent = %d, want %d", plan.PieceLengthExponent, tt.wantExp)
			}
			if plan.ContentSize != tt.wantContent {
				t.Errorf("planned content size = %d, want %d", plan.ContentSize, tt.wantContent)
			}

			res, err := app.CreateTorrent(req)
			if err != nil {
				t.Fatalf("CreateTorrent: %v", err)
			}
			mi, err := torrent.LoadFromFile(res.Path)
			if err != nil {
				t.Fatalf("load written torrent: %v", err)
			}
			if got := mi.GetInfo().PieceLength; got != plan.PieceLengthBytes {
				t.Errorf("written piece length = %d, planned %d", got, plan.PieceLengthBytes)
			}
			if res.Size != plan.ContentSize {
				t.Errorf("written size = %d, planned %d", res.Size, plan.ContentSize)
			}
		})
	}
}

// TestSavePresetLoadsInCLI saves presets with every option set through
// App.SavePreset and loads them as the CLI does. The CLI must get the same values.
func TestSavePresetLoadsInCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Chdir(dir)

	all := preset.Options{
		Private:             new(false),
		NoDate:              new(true),
		NoCreator:           new(true),
		SkipPrefix:          new(true),
		Entropy:             new(true),
		FailOnSeasonWarning: new(true),
		Comment:             "comment",
		Source:              "SRC",
		OutputDir:           "/out",
		Trackers:            []string{"https://tracker.example.invalid/announce"},
		WebSeeds:            []string{"https://seed.example.invalid/files/"},
		ExcludePatterns:     []string{"*.nfo", "*sample*"},
		IncludePatterns:     []string{"*.mkv"},
		PieceLength:         20,
		MaxPieceLength:      24,
		TargetPieceCount:    1000,
		Workers:             4,
	}
	for f, v := range reflect.ValueOf(all).Fields() {
		if v.IsZero() {
			t.Fatalf("set %s in the test preset, so that the test covers it", f.Name)
		}
	}

	// A preset cannot have both a piece length and a target piece count.
	withPieceLength, withTarget := all, all
	withPieceLength.TargetPieceCount = 0
	withTarget.PieceLength = 0

	for name, want := range map[string]preset.Options{"piece-length": withPieceLength, "target": withTarget} {
		if err := (&App{}).SavePreset(name, want); err != nil {
			t.Fatal(err)
		}
		got, err := preset.LoadPresetOptions("", name)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*got, want) {
			t.Errorf("preset %s: loaded %+v, want %+v", name, *got, want)
		}
	}
}

// The editor saves the values that it loads, so it must load what the preset
// sets itself. A value that comes from the default section stays out of Own.
func TestGetAllPresetsOwnValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Chdir(dir)

	yaml := `version: 1
default:
  private: false
  webseeds: [https://seed.example.invalid/]
presets:
  p:
    source: SRC
`
	if err := os.WriteFile("presets.yaml", []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := (&App{}).GetAllPresets()
	if err != nil {
		t.Fatal(err)
	}

	merged := result.Presets["p"]
	own, ok := result.Own["p"]
	if merged == nil || !ok {
		t.Fatalf("preset p missing: presets=%v own=%v", result.Presets, result.Own)
	}
	if len(merged.WebSeeds) != 1 || merged.Private == nil || *merged.Private {
		t.Errorf("merged preset lost the default section: %+v", merged)
	}
	if own.WebSeeds != nil || own.Private != nil || own.Source != "SRC" {
		t.Errorf("own values = %+v, want only source SRC", own)
	}
	// A new preset shows these switch values, so they must come from the default section.
	if d := result.Defaults; d == nil || d.Private == nil || *d.Private {
		t.Errorf("defaults = %+v, want private false from the default section", d)
	}
}

// TestGetAllowedPieceLengthsMatchesCreate checks that the create page lists
// offer exactly the piece lengths that create accepts.
func TestGetAllowedPieceLengthsMatchesCreate(t *testing.T) {
	tests := []struct {
		name          string
		trackerURL    string
		wantPiece     PieceLengthRange
		wantMax       PieceLengthRange
		wantMaxTarget PieceLengthRange
	}{
		{"no tracker", "", PieceLengthRange{15, 27}, PieceLengthRange{15, 27}, PieceLengthRange{16, 27}},
		{"default table", "https://beyond-hd.me/announce?passkey=123", PieceLengthRange{16, 24}, PieceLengthRange{16, 24}, PieceLengthRange{16, 24}},
		{"default table with a lower cap", "https://morethantv.me/announce", PieceLengthRange{16, 23}, PieceLengthRange{16, 23}, PieceLengthRange{16, 23}},
		{"own table", "https://portugas.org/announce/passkey", PieceLengthRange{14, 27}, PieceLengthRange{14, 27}, PieceLengthRange{16, 27}},
		{"own table with a cap", "https://gazellegames.net/announce?passkey=123", PieceLengthRange{14, 26}, PieceLengthRange{14, 26}, PieceLengthRange{16, 26}},
	}

	dir := t.TempDir()
	content := filepath.Join(dir, "content.bin")
	writeFile(t, content, 1<<20)
	app := NewApp("test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.GetAllowedPieceLengths(tt.trackerURL, 0)
			gotTarget := app.GetAllowedPieceLengths(tt.trackerURL, 100)
			if got.PieceLength != tt.wantPiece || got.MaxPieceLength != tt.wantMax || gotTarget.MaxPieceLength != tt.wantMaxTarget {
				t.Fatalf("GetAllowedPieceLengths() = %+v, with target %+v; want piece %v, max %v, max with target %v",
					got, gotTarget, tt.wantPiece, tt.wantMax, tt.wantMaxTarget)
			}

			var trackerURLs []string
			if tt.trackerURL != "" {
				trackerURLs = []string{tt.trackerURL}
			}
			fields := []struct {
				label    string
				allowed  PieceLengthRange
				settings func(exp uint) torrent.CreateSettings
			}{
				{"piece length", got.PieceLength, func(exp uint) torrent.CreateSettings {
					return torrent.CreateSettings{PieceLengthExp: new(exp)}
				}},
				{"max piece length", got.MaxPieceLength, func(exp uint) torrent.CreateSettings {
					return torrent.CreateSettings{MaxPieceLength: new(exp)}
				}},
				{"max piece length with target piece count", gotTarget.MaxPieceLength, func(exp uint) torrent.CreateSettings {
					return torrent.CreateSettings{MaxPieceLength: new(exp), TargetPieceCount: new(uint(100))}
				}},
			}
			for _, f := range fields {
				for exp := uint(10); exp <= 28; exp++ {
					settings := f.settings(exp)
					settings.TrackerURLs = trackerURLs
					_, err := app.PlanCreate(CreateRequest{Path: content, CreateSettings: settings})
					allowed := exp >= f.allowed.Min && exp <= f.allowed.Max
					if allowed && err != nil {
						t.Errorf("%s %d is in the list, but create rejects it: %v", f.label, exp, err)
					}
					if !allowed && err == nil {
						t.Errorf("%s %d is not in the list, but create accepts it", f.label, exp)
					}
				}
			}
		})
	}
}

func TestGetPieceLengthRange(t *testing.T) {
	if got, want := NewApp("test").GetPieceLengthRange(), (PieceLengthRange{14, 27}); got != want {
		t.Fatalf("GetPieceLengthRange() = %v, want %v", got, want)
	}
}
