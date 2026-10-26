// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/mkbrr/torrent"
)

// writeModifyFixture writes a valid torrent and a file that is not a torrent, and returns
// their paths and an output directory.
func writeModifyFixture(t *testing.T) (goodTorrent, brokenTorrent, outputDir string) {
	t.Helper()

	dir := t.TempDir()
	content := filepath.Join(dir, "content.bin")
	require.NoError(t, os.WriteFile(content, make([]byte, 64*1024), 0o644))

	goodTorrent = filepath.Join(dir, "good.torrent")
	_, err := torrent.Create(torrent.CreateOptions{Path: content, OutputPath: goodTorrent, IsPrivate: true, NoDate: true})
	require.NoError(t, err)

	brokenTorrent = filepath.Join(dir, "broken.torrent")
	require.NoError(t, os.WriteFile(brokenTorrent, []byte("not a torrent"), 0o644))

	return goodTorrent, brokenTorrent, filepath.Join(dir, "out")
}

// runModifyCommand runs "mkbrr modify" with args the way a user does, and resets
// the modify flags afterwards so that the next run starts clean.
func runModifyCommand(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Cleanup(func() {
		modifyOpts = modifyOptions{Private: true}
		modifyCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	})

	stdout, stderr = captureOutput(t, func() {
		rootCmd.SetArgs(append([]string{"modify"}, args...))
		err = rootCmd.Execute()
	})
	return stdout, stderr, err
}

func TestModifyReturnsErrorWhenAFileFails(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			goodTorrent, brokenTorrent, outputDir := writeModifyFixture(t)
			args := []string{goodTorrent, brokenTorrent, "--comment", "new comment", "--output-dir", outputDir}
			if quiet {
				args = append(args, "--quiet")
			}

			stdout, stderr, err := runModifyCommand(t, args...)

			require.EqualError(t, err, "1 of 2 torrent files failed")
			written, globErr := filepath.Glob(filepath.Join(outputDir, "*.torrent"))
			require.NoError(t, globErr)
			require.Len(t, written, 1, "files that succeeded keep their output")
			if quiet {
				assert.Contains(t, stdout, "Wrote: "+written[0])
				assert.Contains(t, stderr, "Failed: "+brokenTorrent+": ")
			}
		})
	}
}

func TestModifySucceedsWhenAllFilesPass(t *testing.T) {
	goodTorrent, _, outputDir := writeModifyFixture(t)

	stdout, stderr, err := runModifyCommand(t, goodTorrent, "--comment", "new comment", "--output-dir", outputDir, "--quiet")

	require.NoError(t, err)
	assert.Contains(t, stdout, "Wrote: "+outputDir)
	assert.Empty(t, stderr)
}
