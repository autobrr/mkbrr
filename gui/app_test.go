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
}
