// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"encoding/json/v2"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/torrent"
)

type schemaObject struct {
	file   string
	name   string
	path   []string
	fields map[string]string
}

func TestSchemaKeys(t *testing.T) {
	presetFields := yamlKeyFields(reflect.TypeFor[preset.Options]())
	objects := []schemaObject{
		{"schema/presets.json", "default", []string{"properties", "default"}, presetFields},
		{"schema/presets.json", "presets.<name>", []string{"properties", "presets", "additionalProperties"}, presetFields},
		{"schema/batch.json", "jobs[]", []string{"properties", "jobs", "items"}, yamlKeyFields(reflect.TypeFor[torrent.BatchJob]())},
	}

	for _, o := range objects {
		where := o.file + " " + o.name
		t.Run(where, func(t *testing.T) {
			props := schemaProperties(t, o.file, o.path)
			for _, key := range slices.Sorted(maps.Keys(o.fields)) {
				_, ok := props[key]
				assert.Truef(t, ok, "%s has no key %q (%s)", where, key, o.fields[key])
			}
			for _, key := range slices.Sorted(maps.Keys(props)) {
				_, ok := o.fields[key]
				assert.Truef(t, ok, "%s has key %q, but no Go field reads it", where, key)
			}
		})
	}
}

func schemaProperties(t *testing.T, file string, path []string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../" + file)
	require.NoError(t, err)
	var node map[string]any
	require.NoError(t, json.Unmarshal(data, &node))
	steps := slices.Concat(path, []string{"properties"})
	for i, p := range steps {
		var ok bool
		node, ok = node[p].(map[string]any)
		require.Truef(t, ok, "%s has no object at %s", file, strings.Join(steps[:i+1], "/"))
	}
	return node
}

func yamlKeyFields(typ reflect.Type) map[string]string {
	keys := map[string]string{}
	for f := range typ.Fields() {
		key, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		switch {
		case opts == "inline":
			maps.Copy(keys, yamlKeyFields(f.Type))
		case key != "" && key != "-":
			keys[key] = typ.String() + "." + f.Name
		}
	}
	return keys
}
