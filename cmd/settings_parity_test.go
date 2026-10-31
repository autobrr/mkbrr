// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/torrent"
)

// The places where a setting can be available.
const (
	placeCLI     = "CLI flag"
	placeBatch   = "batch key"
	placePreset  = "preset key"
	placeGUIForm = "GUI form control"
)

// The GUI lists in this file are kept by hand from gui/frontend/src/pages.
// When you add or remove a GUI control, update the list.

// presetEditorKeys are the preset keys that the GUI preset editor (Settings.tsx) has a control for.
var presetEditorKeys = []string{
	"source", "comment", "private", "no_date", "no_creator", "entropy", "skip_prefix",
	"fail_on_season_warning", "trackers", "webseeds", "output_dir", "piece_length",
	"max_piece_length", "target_piece_count", "exclude_patterns", "include_patterns", "workers",
}

type settingsParity struct {
	settings reflect.Type
	cmd      *cobra.Command
	batch    bool     // a batch job embeds the settings
	guiForm  []string // fields that the GUI form has a control for
	// exceptions are the fields that must not be in a place.
	exceptions map[string][]string
	// knownGaps are the fields that are missing from a place now.
	// Remove a field when it is added to the place.
	knownGaps map[string][]string
}

var settingsParityCases = map[string]settingsParity{
	"create": {
		settings: reflect.TypeFor[torrent.CreateSettings](),
		cmd:      createCmd,
		batch:    true,
		guiForm: []string{
			"PieceLengthExp", "MaxPieceLength", "TargetPieceCount", "Comment", "Source", "OutputDir",
			"IsPrivate", "NoDate", "NoCreator", "Entropy", "SkipPrefix", "FailOnSeasonPackWarning",
			"TrackerURLs", "WebSeeds", "ExcludePatterns", "IncludePatterns",
		},
		exceptions: map[string][]string{
			// A batch file sets the output path of each job. Batch mode uses the automatic worker count.
			placeBatch: {"OutputDir", "Workers"},
			// The GUI settings page sets the default workers for all torrents.
			placeGUIForm: {"Workers"},
		},
	},
	"modify": {
		settings: reflect.TypeFor[torrent.ModifySettings](),
		cmd:      modifyCmd,
		guiForm: []string{
			"TrackerURLs", "WebSeeds", "Name", "Comment", "Source", "IsPrivate", "NoPrivate", "NoDate",
			"NoCreator", "Entropy", "NoEntropy", "OutputDir", "OutputPattern", "SkipPrefix",
		},
		exceptions: map[string][]string{
			// A preset cannot clear, and the name and the output filename belong to one torrent.
			placePreset: {"Name", "NoPrivate", "NoEntropy", "OutputPattern"},
		},
	},
}

// TestSettingsParity fails when a create or modify setting is not available
// in each place where it must be: the CLI, batch, presets, and the GUI.
func TestSettingsParity(t *testing.T) {
	presetKeys := yamlKeys(reflect.TypeFor[preset.Options]())
	usedPresetKeys := map[string]bool{}

	for name, tc := range settingsParityCases {
		t.Run(name, func(t *testing.T) {
			check := func(place, field string, found bool) {
				t.Helper()
				switch {
				case slices.Contains(tc.exceptions[place], field):
					assert.Falsef(t, found, "%s has a %s, but it is in the exceptions list for that place; remove it from the list", field, place)
				case slices.Contains(tc.knownGaps[place], field):
					assert.Falsef(t, found, "%s has a %s now; remove it from the known gaps list", field, place)
				default:
					assert.Truef(t, found, "%s has no %s; add one, or add the field to the exceptions or known gaps list", field, place)
				}
			}

			var fields []string
			for f := range tc.settings.Fields() {
				fields = append(fields, f.Name)
				check(placeCLI, f.Name, tc.cmd.Flags().Lookup(f.Tag.Get("flag")) != nil)

				if tc.batch {
					key, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
					check(placeBatch, f.Name, key != "" && key != "-")
				}

				key := f.Tag.Get("preset")
				check(placePreset, f.Name, slices.Contains(presetKeys, key))
				if key != "" {
					usedPresetKeys[key] = true
				}

				check(placeGUIForm, f.Name, slices.Contains(tc.guiForm, f.Name))
			}

			listed := slices.Clone(tc.guiForm)
			for _, names := range tc.exceptions {
				listed = append(listed, names...)
			}
			for _, names := range tc.knownGaps {
				listed = append(listed, names...)
			}
			for _, n := range listed {
				assert.Containsf(t, fields, n, "the test lists %s, but the settings type has no such field", n)
			}
		})
	}

	for _, key := range slices.Sorted(maps.Keys(usedPresetKeys)) {
		assert.Containsf(t, presetEditorKeys, key, "the GUI preset editor has no control for preset key %q; add one, and add the key to presetEditorKeys", key)
	}
	for _, key := range presetEditorKeys {
		assert.Truef(t, usedPresetKeys[key], "preset key %q is in a preset editor list, but no setting has it", key)
	}
}

func yamlKeys(typ reflect.Type) []string {
	var keys []string
	for f := range typ.Fields() {
		key, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		keys = append(keys, key)
	}
	return keys
}
