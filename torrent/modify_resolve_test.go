// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModify(t *testing.T) {
	keep := ModifyChange{Action: ModifyKeep, Origin: OriginKeep}
	keepText := ModifyTextChange{Action: ModifyKeep, Origin: OriginKeep}
	keepFlag := ModifyFlagChange{Action: ModifyKeep, Origin: OriginKeep}
	keepList := ModifyListChange{Action: ModifyKeep, Origin: OriginKeep}
	keepAll := ModifyResolution{
		Trackers: keepList, WebSeeds: keepList, Name: keepText, Comment: keepText, Source: keepText,
		Private: keepFlag, CreatedBy: keepText, CreationDate: keep, Entropy: keep, OutputDir: keepText, SkipPrefix: keepFlag,
	}
	with := func(change func(*ModifyResolution)) ModifyResolution {
		r := keepAll
		change(&r)
		return r
	}

	tests := []struct {
		name       string
		presetYAML string
		settings   ModifySettings
		want       ModifyResolution
	}{
		{
			name: "no preset and no override keeps everything",
			want: keepAll,
		},
		{
			name:       "preset without private keeps the private flag",
			presetYAML: "{trackers: [http://tracker.invalid/announce]}",
			want: with(func(r *ModifyResolution) {
				r.Trackers = ModifyListChange{Action: ModifySet, Origin: OriginPreset, Value: []string{"http://tracker.invalid/announce"}}
			}),
		},
		{
			name:       "preset values",
			presetYAML: "{source: PRESET, comment: preset, private: false, entropy: true, no_creator: false, no_date: false, skip_prefix: true, output_dir: out, webseeds: [http://seed.invalid/]}",
			want: with(func(r *ModifyResolution) {
				r.WebSeeds = ModifyListChange{Action: ModifySet, Origin: OriginPreset, Value: []string{"http://seed.invalid/"}}
				r.Source = ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: "PRESET"}
				r.Comment = ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: "preset"}
				r.Private = ModifyFlagChange{Action: ModifySet, Origin: OriginPreset, Value: false}
				r.Entropy = ModifyChange{Action: ModifySet, Origin: OriginPreset}
				r.CreatedBy = ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: createdBy("1")}
				r.CreationDate = ModifyChange{Action: ModifyKeep, Origin: OriginPreset}
				r.SkipPrefix = ModifyFlagChange{Action: ModifySet, Origin: OriginPreset, Value: true}
				r.OutputDir = ModifyTextChange{Action: ModifySet, Origin: OriginPreset, Value: "out"}
			}),
		},
		{
			name:       "a preset keeps with entropy false and clears with no_date and no_creator",
			presetYAML: "{entropy: false, no_date: true, no_creator: true}",
			want: with(func(r *ModifyResolution) {
				r.Entropy = ModifyChange{Action: ModifyKeep, Origin: OriginPreset}
				r.CreationDate = ModifyChange{Action: ModifyClear, Origin: OriginPreset}
				r.CreatedBy = ModifyTextChange{Action: ModifyClear, Origin: OriginPreset}
			}),
		},
		{
			name:       "overrides win over the preset",
			presetYAML: "{trackers: [http://tracker.invalid/announce], source: PRESET, comment: preset, private: true, entropy: true, output_dir: out}",
			settings: ModifySettings{
				TrackerURLs: []string{"http://typed.invalid/announce"},
				Name:        "typed",
				Source:      new("TYPED"),
				Comment:     new("typed"),
				IsPrivate:   new(false),
				Entropy:     new(false),
				OutputDir:   "typed-out",
			},
			want: with(func(r *ModifyResolution) {
				r.Trackers = ModifyListChange{Action: ModifySet, Origin: OriginOverride, Value: []string{"http://typed.invalid/announce"}}
				r.Name = ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: "typed"}
				r.Source = ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: "TYPED"}
				r.Comment = ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: "typed"}
				r.Private = ModifyFlagChange{Action: ModifySet, Origin: OriginOverride, Value: false}
				r.Entropy = ModifyChange{Action: ModifyKeep, Origin: OriginOverride}
				r.OutputDir = ModifyTextChange{Action: ModifySet, Origin: OriginOverride, Value: "typed-out"}
			}),
		},
		{
			name:       "only an override clears",
			presetYAML: "{trackers: [http://tracker.invalid/announce], webseeds: [http://seed.invalid/], source: PRESET, comment: preset, private: true, entropy: true, no_creator: false}",
			settings: ModifySettings{
				RemoveTrackers: []string{"old.invalid"},
				NoWebSeed:      true,
				Source:         new(""),
				Comment:        new(""),
				NoPrivate:      true,
				NoEntropy:      true,
				NoCreator:      true,
				NoDate:         true,
			},
			want: with(func(r *ModifyResolution) {
				r.Trackers = ModifyListChange{Action: ModifyRemove, Origin: OriginOverride, Value: []string{"old.invalid"}}
				r.WebSeeds = ModifyListChange{Action: ModifyClear, Origin: OriginOverride}
				r.Source = ModifyTextChange{Action: ModifyClear, Origin: OriginOverride}
				r.Comment = ModifyTextChange{Action: ModifyClear, Origin: OriginOverride}
				r.Private = ModifyFlagChange{Action: ModifyClear, Origin: OriginOverride}
				r.Entropy = ModifyChange{Action: ModifyClear, Origin: OriginOverride}
				r.CreatedBy = ModifyTextChange{Action: ModifyClear, Origin: OriginOverride}
				r.CreationDate = ModifyChange{Action: ModifyClear, Origin: OriginOverride}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ModifyOptions{ModifySettings: tt.settings, Version: "1"}
			if tt.presetYAML != "" {
				opts.PresetName = "p"
				opts.PresetFile = filepath.Join(t.TempDir(), "presets.yaml")
				yaml := "version: 1\npresets:\n  p: " + tt.presetYAML + "\n"
				require.NoError(t, os.WriteFile(opts.PresetFile, []byte(yaml), 0o600))
			}

			got, err := ResolveModify(opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveModifyErrors(t *testing.T) {
	_, err := ResolveModify(ModifyOptions{ModifySettings: ModifySettings{NoEntropy: true, Entropy: new(true)}})
	require.ErrorIs(t, err, errEntropyConflict)

	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	_, err = ResolveModify(ModifyOptions{PresetName: "missing", PresetFile: filepath.Join(t.TempDir(), "presets.yaml")})
	require.ErrorContains(t, err, "could not find preset file")
}
