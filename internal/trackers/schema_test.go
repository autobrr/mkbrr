// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package trackers

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaPieceLengthRange checks that the JSON schemas accept the outer
// piece length range, so an editor accepts every value that some tracker allows.
func TestSchemaPieceLengthRange(t *testing.T) {
	for _, file := range []string{"batch.json", "presets.json"} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "schema", file))
			if err != nil {
				t.Fatal(err)
			}
			var schema any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatal(err)
			}

			found := map[string]int{}
			var walk func(node any)
			walk = func(node any) {
				switch n := node.(type) {
				case map[string]any:
					if props, ok := n["properties"].(map[string]any); ok {
						for _, key := range []string{"piece_length", "max_piece_length"} {
							prop, ok := props[key].(map[string]any)
							if !ok {
								continue
							}
							found[key]++
							if prop["minimum"] != float64(MinPieceExp) || prop["maximum"] != float64(MaxPieceExp) {
								t.Errorf("%s: minimum %v and maximum %v, want %d and %d",
									key, prop["minimum"], prop["maximum"], MinPieceExp, MaxPieceExp)
							}
						}
					}
					for _, v := range n {
						walk(v)
					}
				case []any:
					for _, v := range n {
						walk(v)
					}
				}
			}
			walk(schema)

			for _, key := range []string{"piece_length", "max_piece_length"} {
				if found[key] == 0 {
					t.Errorf("no %s property in %s", key, file)
				}
			}
		})
	}
}
