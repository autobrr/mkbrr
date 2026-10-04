// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBatchFixture writes a content file and a batch config with one job that
// succeeds and, if withFailure is set, one whose output directory does not exist.
func writeBatchFixture(t *testing.T, withFailure bool) (batchFile, goodOutput string) {
	t.Helper()

	dir := t.TempDir()
	content := filepath.Join(dir, "content.bin")
	require.NoError(t, os.WriteFile(content, make([]byte, 64*1024), 0o644))

	goodOutput = filepath.Join(dir, "good.torrent")
	config := fmt.Sprintf("version: 1\njobs:\n  - path: %q\n    output: %q\n", content, goodOutput)
	if withFailure {
		badOutput := filepath.Join(dir, "missing-dir", "bad.torrent")
		config += fmt.Sprintf("  - path: %q\n    output: %q\n", content, badOutput)
	}

	batchFile = filepath.Join(dir, "batch.yaml")
	require.NoError(t, os.WriteFile(batchFile, []byte(config), 0o644))
	return batchFile, goodOutput
}

// captureOutput runs fn and returns what it wrote to stdout and stderr.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = outW, errW

	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	fn()

	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	outBytes, err := io.ReadAll(outR)
	require.NoError(t, err)
	errBytes, err := io.ReadAll(errR)
	require.NoError(t, err)
	return string(outBytes), string(errBytes)
}

func TestProcessBatchModeReturnsErrorWhenAJobFails(t *testing.T) {
	batchFile, goodOutput := writeBatchFixture(t, true)

	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			var err error
			stdout, stderr := captureOutput(t, func() {
				err = processBatchMode(createOptions{batchFile: batchFile, quiet: quiet}, "test", time.Now())
			})

			require.EqualError(t, err, "1 of 2 batch jobs failed")
			assert.FileExists(t, goodOutput, "jobs that succeeded keep their output")
			if quiet {
				assert.Contains(t, stdout, "Wrote: "+goodOutput)
				assert.Contains(t, stderr, "Failed:")
				assert.Contains(t, stderr, "failed to create output file")
			}
		})
	}
}

func TestProcessBatchModeSucceedsWhenAllJobsPass(t *testing.T) {
	batchFile, goodOutput := writeBatchFixture(t, false)

	var err error
	captureOutput(t, func() {
		err = processBatchMode(createOptions{batchFile: batchFile, quiet: true}, "test", time.Now())
	})

	require.NoError(t, err)
	assert.FileExists(t, goodOutput)
}
