// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

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
