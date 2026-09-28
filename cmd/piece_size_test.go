// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCalculatePieceSizeResult(t *testing.T) {
	tests := []struct {
		name                string
		size                uint64
		trackerURL          string
		wantExp             uint
		wantSource          string
		wantMaxTorrentBytes uint64
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
		{
			name:                "tracker torrent-size limit is reported",
			size:                3 << 30,
			trackerURL:          "https://portugas.org/announce/passkey",
			wantExp:             21,
			wantSource:          "tracker",
			wantMaxTorrentBytes: 2 << 20,
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
			if got.MaxTorrentBytes != tt.wantMaxTorrentBytes {
				t.Fatalf("max torrent bytes = %d, want %d", got.MaxTorrentBytes, tt.wantMaxTorrentBytes)
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

	if got.ContentBytes != 63<<20 || got.Exponent != 15 || got.Bytes != 1<<15 || got.Source != "default" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestContentSizeFromPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := contentSizeFromPath(dir)
	if err != nil {
		t.Fatalf("contentSizeFromPath() error = %v", err)
	}
	if got != 3072 {
		t.Fatalf("content size = %d, want 3072", got)
	}

	fileSize, err := contentSizeFromPath(filepath.Join(dir, "a.bin"))
	if err != nil {
		t.Fatalf("contentSizeFromPath(file) error = %v", err)
	}
	if fileSize != 1024 {
		t.Fatalf("file size = %d, want 1024", fileSize)
	}
}

func TestRunPieceSizeFileJSON(t *testing.T) {
	old := pieceSizeOpts
	defer func() { pieceSizeOpts = old }()

	dir := t.TempDir()
	file := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(file, 63<<20); err != nil {
		t.Fatal(err)
	}

	pieceSizeOpts = pieceSizeOptions{
		file: file,
		json: true,
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

	if got.ContentBytes != 63<<20 || got.Exponent != 15 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestContentSizeFromDirectorySymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "payload.bin"), make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got, err := contentSizeFromPath(link)
	if err != nil {
		t.Fatalf("contentSizeFromPath(symlink) error = %v", err)
	}
	if got != 4096 {
		t.Fatalf("content size = %d, want 4096", got)
	}
}

func TestRunPieceSizeRejectsExplicitZeroSizeWithFile(t *testing.T) {
	old := pieceSizeOpts
	defer func() { pieceSizeOpts = old }()

	pieceSizeOpts = pieceSizeOptions{
		file: "payload.bin",
	}

	cmd := &cobra.Command{}
	cmd.Flags().Uint64("size", 0, "")
	if err := cmd.Flags().Set("size", "0"); err != nil {
		t.Fatal(err)
	}

	if err := runPieceSize(cmd, nil); err == nil {
		t.Fatal("expected mutually exclusive input error")
	}
}

func TestRunPieceSizeRejectsSizeAndFileTogether(t *testing.T) {
	old := pieceSizeOpts
	defer func() { pieceSizeOpts = old }()

	pieceSizeOpts = pieceSizeOptions{
		size: 1,
		file: "payload.bin",
	}

	if err := runPieceSize(&cobra.Command{}, nil); err == nil {
		t.Fatal("expected mutually exclusive input error")
	}
}
