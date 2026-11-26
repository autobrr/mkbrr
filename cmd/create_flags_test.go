// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/mkbrr/torrent"
)

var createFlagValues = map[string]string{
	"piece-length":           "18",
	"max-piece-length":       "22",
	"target-piece-count":     "1000",
	"comment":                "flag comment",
	"source":                 "FLAG",
	"output-dir":             "/flag-out",
	"workers":                "2",
	"private":                "false",
	"no-date":                "true",
	"no-creator":             "true",
	"entropy":                "true",
	"skip-prefix":            "true",
	"fail-on-season-warning": "true",
	"tracker":                "https://tracker.example.invalid/announce",
	"web-seed":               "https://seed.example.invalid/",
	"exclude":                "*.nfo",
	"include":                "*.mkv",
}

func TestCreateFlagWiring(t *testing.T) {
	baseline := reflect.ValueOf(resolveCreateFlags(t))

	for f := range reflect.TypeFor[torrent.CreateSettings]().Fields() {
		name := f.Tag.Get("flag")
		t.Run(name, func(t *testing.T) {
			value, ok := createFlagValues[name]
			require.Truef(t, ok, "add a value for --%s to createFlagValues", name)

			got := reflect.ValueOf(resolveCreateFlags(t, "--"+name+"="+value)).FieldByName(f.Name)
			require.Truef(t, got.IsValid(), "torrent.CreateOptions has no field %s", f.Name)
			assert.NotEqualf(t, baseline.FieldByName(f.Name).Interface(), got.Interface(),
				"--%s=%s did not change %s; copy the flag into its setting in buildCreateOptions", name, value, f.Name)
		})
	}
}
