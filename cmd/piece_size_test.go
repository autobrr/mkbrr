// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestCalculatePieceSizeResult(t *testing.T) {
	tests := []struct {
		name       string
		size       uint64
		trackerURL string
		wantExp    uint
		wantSource string
	}{
		{
			name:       "known tracker uses tracker policy",
			size:       3 << 30,
			trackerURL: "https://gazellegames.net/announce?passkey=123",
			wantExp:    21,
			wantSource: "tracker",
		},
		{
			name:       "unknown tracker uses default policy",
			size:       63 << 20,
			trackerURL: "https://unknown.tracker/announce",
			wantExp:    15,
			wantSource: "default",
		},
		{
			name:       "no tracker uses default policy",
			size:       63 << 20,
			wantExp:    15,
			wantSource: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculatePieceSizeResult(tt.size, tt.trackerURL)
			if err != nil {
				t.Fatalf("calculatePieceSizeResult() error = %v", err)
			}
			if got.Exponent != tt.wantExp {
				t.Fatalf("exponent = %d, want %d", got.Exponent, tt.wantExp)
			}
			if got.Source != tt.wantSource {
				t.Fatalf("source = %q, want %q", got.Source, tt.wantSource)
			}
			if got.Bytes != uint64(1)<<got.Exponent {
				t.Fatalf("bytes = %d, want %d", got.Bytes, uint64(1)<<got.Exponent)
			}
		})
	}
}

func TestCalculatePieceSizeResultRejectsZero(t *testing.T) {
	if _, err := calculatePieceSizeResult(0, ""); err == nil {
		t.Fatal("expected zero-size error")
	}
}

func TestRunPieceSizeJSON(t *testing.T) {
	old := pieceSizeOpts
	defer func() { pieceSizeOpts = old }()

	pieceSizeOpts = pieceSizeOptions{
		size:    63 << 20,
		tracker: "https://unknown.tracker/announce",
		json:    true,
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := runPieceSize(cmd, nil); err != nil {
		t.Fatalf("runPieceSize() error = %v", err)
	}

	var got pieceSizeResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if got.Exponent != 15 || got.Bytes != 1<<15 || got.Source != "default" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
