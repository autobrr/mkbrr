// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"path/filepath"
	"testing"
)

func TestParseLaunchArgs(t *testing.T) {
	workDir := t.TempDir()
	absTorrent := filepath.Join(t.TempDir(), "abs.torrent")

	tests := []struct {
		name string
		args []string
		want *LaunchCommand
	}{
		{name: "no arguments", args: nil, want: nil},
		{name: "unknown command", args: []string{"foo", "file.torrent"}, want: nil},
		{name: "inspect without path", args: []string{"inspect"}, want: &LaunchCommand{Command: "inspect"}},
		{
			name: "inspect with relative path",
			args: []string{"inspect", "file.torrent"},
			want: &LaunchCommand{Command: "inspect", Path: filepath.Join(workDir, "file.torrent")},
		},
		{
			name: "inspect with absolute path",
			args: []string{"inspect", absTorrent},
			want: &LaunchCommand{Command: "inspect", Path: absTorrent},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLaunchArgs(tt.args, workDir)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("parseLaunchArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
