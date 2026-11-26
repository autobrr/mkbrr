// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"

	"github.com/autobrr/mkbrr/internal/createpreset"
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
			if res.Path != plan.OutputPath {
				t.Errorf("written path = %q, planned %q", res.Path, plan.OutputPath)
			}
			if got, want := filepath.Dir(plan.OutputPath), filepath.Dir(req.Path); got != want {
				t.Errorf("planned output folder = %q, want the source folder %q", got, want)
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
	loaded := torrent.CreateSettings{
		IsPrivate:               new(false),
		NoDate:                  new(true),
		NoCreator:               new(true),
		SkipPrefix:              new(true),
		Entropy:                 new(true),
		FailOnSeasonPackWarning: new(true),
		Comment:                 new("comment"),
		Source:                  new("SRC"),
		OutputDir:               new("/out"),
		TrackerURLs:             []string{"https://tracker.example.invalid/announce"},
		WebSeeds:                []string{"https://seed.example.invalid/files/"},
		ExcludePatterns:         []string{"*.nfo", "*sample*"},
		IncludePatterns:         []string{"*.mkv"},
		PieceLengthExp:          new(uint(20)),
		MaxPieceLength:          new(uint(24)),
		TargetPieceCount:        new(uint(1000)),
		Workers:                 new(4),
	}
	for _, v := range []reflect.Value{reflect.ValueOf(all), reflect.ValueOf(loaded)} {
		for f, fv := range v.Fields() {
			if fv.IsZero() {
				t.Fatalf("set %s in the test preset, so that the test covers it", f.Name)
			}
		}
	}

	// A preset cannot have both a piece length and a target piece count.
	saveWithPieceLength, saveWithTarget := all, all
	saveWithPieceLength.TargetPieceCount = 0
	saveWithTarget.PieceLength = 0
	wantWithPieceLength, wantWithTarget := loaded, loaded
	wantWithPieceLength.TargetPieceCount = nil
	wantWithTarget.PieceLengthExp = nil

	cases := map[string]struct {
		save preset.Options
		want torrent.CreateSettings
	}{
		"piece-length": {saveWithPieceLength, wantWithPieceLength},
		"target":       {saveWithTarget, wantWithTarget},
	}
	for name, tc := range cases {
		if err := (&App{}).SavePreset(name, tc.save); err != nil {
			t.Fatal(err)
		}
		got, err := createpreset.Load("", name)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*got, tc.want) {
			t.Errorf("preset %s: loaded %+v, want %+v", name, *got, tc.want)
		}
		resolved, err := (&App{}).ResolveCreate(CreateRequest{PresetName: name})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(resolved.Settings, tc.want) {
			t.Errorf("preset %s: the Create page shows %+v, want %+v", name, resolved.Settings, tc.want)
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

func TestGetPresetValuesPrivate(t *testing.T) {
	tracker := "trackers: [http://tracker.invalid/announce]"
	cases := []struct {
		name        string
		defaultYAML string
		presetYAML  string
		want        *bool
	}{
		{name: "no private", presetYAML: tracker},
		{name: "private true", presetYAML: tracker + "\n    private: true", want: new(true)},
		{name: "private false", presetYAML: tracker + "\n    private: false", want: new(false)},
		{name: "private false in default section", defaultYAML: "default:\n  private: false\n", presetYAML: tracker, want: new(false)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Chdir(dir)
			yaml := "version: 1\n" + tc.defaultYAML + "presets:\n  p:\n    " + tc.presetYAML + "\n"
			if err := os.WriteFile("presets.yaml", []byte(yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			got, err := (&App{}).GetPresetValues("p")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Private, tc.want) {
				t.Errorf("private = %v, want %v", ptrString(got.Private), ptrString(tc.want))
			}
			if len(got.Trackers) != 1 {
				t.Errorf("trackers = %v, want the preset tracker", got.Trackers)
			}

			create, err := (&App{}).GetPreset("p")
			if err != nil {
				t.Fatal(err)
			}
			wantCreate := tc.want
			if wantCreate == nil {
				wantCreate = new(true)
			}
			if !reflect.DeepEqual(create.Private, wantCreate) {
				t.Errorf("create private = %v, want %v", ptrString(create.Private), ptrString(wantCreate))
			}
		})
	}
}

func ptrString(b *bool) string {
	if b == nil {
		return "unset"
	}
	return strconv.FormatBool(*b)
}

func TestResolveCreateAllowedPieceLengthsMatchCreate(t *testing.T) {
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
			var trackerURLs []string
			if tt.trackerURL != "" {
				trackerURLs = []string{tt.trackerURL}
			}
			resolved, err := app.ResolveCreate(CreateRequest{TrackerURLs: trackerURLs})
			if err != nil {
				t.Fatal(err)
			}
			resolvedTarget, err := app.ResolveCreate(CreateRequest{TrackerURLs: trackerURLs, TargetPieceCount: new(uint(100))})
			if err != nil {
				t.Fatal(err)
			}
			got, gotTarget := resolved.AllowedPieceLengths, resolvedTarget.AllowedPieceLengths
			if got.PieceLength != tt.wantPiece || got.MaxPieceLength != tt.wantMax || gotTarget.MaxPieceLength != tt.wantMaxTarget {
				t.Fatalf("allowed piece lengths = %+v, with target %+v; want piece %v, max %v, max with target %v",
					got, gotTarget, tt.wantPiece, tt.wantMax, tt.wantMaxTarget)
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

func TestGUICreateOutputPathsUnchanged(t *testing.T) {
	const tracker = "https://tracker.example.org/announce"

	t.Run("with output folder", func(t *testing.T) {
		dir := t.TempDir()
		content := filepath.Join(dir, "Movie")
		writeFile(t, filepath.Join(content, "main.mkv"), 64<<10)
		outDir := filepath.Join(dir, "torrents")
		app := NewApp("test")

		res, err := app.CreateTorrent(CreateRequest{
			Path: content,
			CreateSettings: torrent.CreateSettings{
				OutputDir:   &outDir,
				TrackerURLs: []string{tracker},
			},
		})
		if err != nil {
			t.Fatalf("CreateTorrent: %v", err)
		}
		want := filepath.Join(outDir, "example_Movie.torrent")
		if res.Path != want {
			t.Errorf("path = %q, want %q", res.Path, want)
		}
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected torrent at %q: %v", want, err)
		}
	})

	t.Run("without output folder writes next to source", func(t *testing.T) {
		dir := t.TempDir()
		content := filepath.Join(dir, "Movie")
		writeFile(t, filepath.Join(content, "main.mkv"), 64<<10)
		app := NewApp("test")

		res, err := app.CreateTorrent(CreateRequest{
			Path: content,
			CreateSettings: torrent.CreateSettings{
				TrackerURLs: []string{tracker},
			},
		})
		if err != nil {
			t.Fatalf("CreateTorrent: %v", err)
		}
		want := filepath.Join(dir, "example_Movie.torrent")
		if res.Path != want {
			t.Errorf("path = %q, want %q", res.Path, want)
		}
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected torrent at %q: %v", want, err)
		}
	})
}

func TestResolveCreateMatchesCreateTorrent(t *testing.T) {
	const ptpTracker = "https://passthepopcorn.me/announce?passkey=123"
	const otherTracker = "http://tracker.invalid/announce"

	tests := []struct {
		name        string
		presetYAML  string
		settings    torrent.CreateSettings
		wantOrigins map[string]torrent.Origin
		wantFiles   int
	}{
		{
			name: "typed values",
			settings: torrent.CreateSettings{
				TrackerURLs:    []string{otherTracker, "http://backup.invalid/announce"},
				Comment:        new("typed comment"),
				Source:         new("TYPED"),
				IsPrivate:      new(false),
				NoDate:         new(true),
				Entropy:        new(true),
				SkipPrefix:     new(true),
				PieceLengthExp: new(uint(16)),
			},
			wantOrigins: map[string]torrent.Origin{
				"comment": torrent.OriginOverride, "source": torrent.OriginOverride, "private": torrent.OriginOverride,
				"trackers": torrent.OriginOverride, "piece length": torrent.OriginOverride, "no creator": torrent.OriginDefault,
			},
		},
		{
			name: "preset values",
			presetYAML: `    trackers: [` + otherTracker + `]
    webseeds: [http://seed.invalid/files/]
    comment: preset comment
    source: PRESET
    private: false
    no_creator: true
    max_piece_length: 16
    output_dir: OUTDIR
`,
			wantOrigins: map[string]torrent.Origin{
				"comment": torrent.OriginPreset, "source": torrent.OriginPreset, "private": torrent.OriginPreset,
				"trackers": torrent.OriginPreset, "no creator": torrent.OriginPreset, "output dir": torrent.OriginPreset,
				"no date": torrent.OriginDefault, "piece length": torrent.OriginDefault,
			},
		},
		{
			name:       "tracker default source",
			presetYAML: "    trackers: [\"" + ptpTracker + "\"]\n",
			wantOrigins: map[string]torrent.Origin{
				"source": torrent.OriginTrackerDefault, "trackers": torrent.OriginPreset, "private": torrent.OriginDefault,
			},
		},
		{
			name:       "typed values over the preset",
			presetYAML: "    trackers: [\"" + ptpTracker + "\"]\n    comment: preset comment\n    target_piece_count: 4\n",
			settings:   torrent.CreateSettings{Comment: new("typed comment"), PieceLengthExp: new(uint(15))},
			wantOrigins: map[string]torrent.Origin{
				"comment": torrent.OriginOverride, "source": torrent.OriginTrackerDefault, "piece length": torrent.OriginOverride,
				"target piece count": torrent.OriginDefault,
			},
		},
		{
			name:       "typed patterns add to the preset patterns",
			presetYAML: "    exclude_patterns: [\"*.nfo\"]\n",
			settings:   torrent.CreateSettings{ExcludePatterns: []string{"*.jpg"}},
			wantOrigins: map[string]torrent.Origin{
				"exclude patterns": torrent.OriginPreset,
			},
			wantFiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			content := filepath.Join(dir, "Movie")
			writeFile(t, filepath.Join(content, "main.mkv"), 256<<10)
			writeFile(t, filepath.Join(content, "info.nfo"), 1<<10)
			writeFile(t, filepath.Join(content, "cover.jpg"), 1<<10)
			req := CreateRequest{Path: content, CreateSettings: tt.settings}
			if tt.presetYAML != "" {
				req.PresetName = "p"
				req.PresetFile = filepath.Join(dir, "presets.yaml")
				yaml := "version: 1\npresets:\n  p:\n" + strings.ReplaceAll(tt.presetYAML, "OUTDIR", filepath.Join(dir, "out"))
				if err := os.WriteFile(req.PresetFile, []byte(yaml), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			app := NewApp("test")

			resolved, err := app.ResolveCreate(req)
			if err != nil {
				t.Fatalf("ResolveCreate: %v", err)
			}
			origins := map[string]torrent.Origin{
				"comment": resolved.Origins.Comment, "source": resolved.Origins.Source, "private": resolved.Origins.IsPrivate,
				"trackers": resolved.Origins.TrackerURLs, "no date": resolved.Origins.NoDate, "no creator": resolved.Origins.NoCreator,
				"piece length": resolved.Origins.PieceLengthExp, "target piece count": resolved.Origins.TargetPieceCount,
				"output dir": resolved.Origins.OutputDir, "exclude patterns": resolved.Origins.ExcludePatterns,
			}
			for setting, want := range tt.wantOrigins {
				if origins[setting] != want {
					t.Errorf("%s origin = %q, want %q", setting, origins[setting], want)
				}
			}

			res, err := app.CreateTorrent(req)
			if err != nil {
				t.Fatalf("CreateTorrent: %v", err)
			}
			mi, err := torrent.LoadFromFile(res.Path)
			if err != nil {
				t.Fatal(err)
			}
			var info map[string]any
			if err := bencode.Unmarshal(mi.InfoBytes, &info); err != nil {
				t.Fatal(err)
			}
			got := resolved.Settings
			written := torrent.CreateSettings{
				Comment:     new(mi.Comment),
				Source:      new(stringValue(info["source"])),
				IsPrivate:   new(info["private"] == int64(1)),
				NoDate:      new(mi.CreationDate == 0),
				NoCreator:   new(mi.CreatedBy == ""),
				Entropy:     new(info["entropy"] != nil),
				TrackerURLs: slices.Concat(mi.UpvertedAnnounceList()...),
				WebSeeds:    mi.UrlList,
			}
			want := torrent.CreateSettings{
				Comment:     got.Comment,
				Source:      got.Source,
				IsPrivate:   got.IsPrivate,
				NoDate:      got.NoDate,
				NoCreator:   got.NoCreator,
				Entropy:     got.Entropy,
				TrackerURLs: got.TrackerURLs,
				WebSeeds:    got.WebSeeds,
			}
			if !reflect.DeepEqual(written, want) {
				t.Errorf("written settings:\n%s\nresolved settings:\n%s", settingsString(written), settingsString(want))
			}
			if got.PieceLengthExp != nil && mi.GetInfo().PieceLength != 1<<*got.PieceLengthExp {
				t.Errorf("written piece length = %d, resolved exponent %d", mi.GetInfo().PieceLength, *got.PieceLengthExp)
			}
			if got.MaxPieceLength != nil && mi.GetInfo().PieceLength > 1<<*got.MaxPieceLength {
				t.Errorf("written piece length = %d, above the resolved max exponent %d", mi.GetInfo().PieceLength, *got.MaxPieceLength)
			}
			wantDir := *got.OutputDir
			if wantDir == "" {
				wantDir = dir
			}
			if filepath.Dir(res.Path) != wantDir {
				t.Errorf("written to %q, want the folder %q", res.Path, wantDir)
			}
			var included int
			for _, f := range []string{"main.mkv", "info.nfo", "cover.jpg"} {
				if !slices.ContainsFunc(got.ExcludePatterns, func(p string) bool { m, _ := filepath.Match(p, f); return m }) {
					included++
				}
			}
			if res.FileCount != included {
				t.Errorf("written %d files, the resolved exclude patterns %v leave %d", res.FileCount, got.ExcludePatterns, included)
			}
			if tt.wantFiles != 0 && res.FileCount != tt.wantFiles {
				t.Errorf("written %d files, want %d", res.FileCount, tt.wantFiles)
			}
			if len(got.TrackerURLs) > 0 && *got.SkipPrefix != (filepath.Base(res.Path) == "Movie.torrent") {
				t.Errorf("written file name %q, resolved skip prefix %v", filepath.Base(res.Path), *got.SkipPrefix)
			}
		})
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func settingsString(s torrent.CreateSettings) string {
	return fmt.Sprintf("comment %q source %q private %v no date %v no creator %v entropy %v trackers %v web seeds %v",
		*s.Comment, *s.Source, *s.IsPrivate, *s.NoDate, *s.NoCreator, *s.Entropy, s.TrackerURLs, s.WebSeeds)
}
