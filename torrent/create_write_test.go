// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var oldTorrentBytes = []byte("old torrent bytes")

func writeOldTorrent(t *testing.T, opts CreateOptions) {
	t.Helper()
	require.NoError(t, os.WriteFile(opts.OutputPath, oldTorrentBytes, 0o644))
}

func assertOldTorrent(t *testing.T, opts CreateOptions) {
	t.Helper()
	got, err := os.ReadFile(opts.OutputPath)
	require.NoError(t, err)
	assert.Equal(t, oldTorrentBytes, got)
}

func assertOnlyOutputInFolder(t *testing.T, opts CreateOptions) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(opts.OutputPath))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Base(opts.OutputPath), entries[0].Name())
}

func TestCreateReplacesExistingFile(t *testing.T) {
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)

	_, err := CreateContext(t.Context(), opts)
	require.NoError(t, err)

	got, err := os.ReadFile(opts.OutputPath)
	require.NoError(t, err)
	assert.NotEqual(t, oldTorrentBytes, got)
	assertOnlyOutputInFolder(t, opts)
}

func TestCreateKeepsExistingFileWhenPublishFails(t *testing.T) {
	prev := publishCreatedTorrent
	t.Cleanup(func() { publishCreatedTorrent = prev })
	publishErr := errors.New("disk full")
	var tempPath string
	publishCreatedTorrent = func(temp, _ string, _ bool) error {
		tempPath = temp
		return publishErr
	}
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)

	_, err := CreateContext(t.Context(), opts)

	require.ErrorIs(t, err, publishErr)
	assertOldTorrent(t, opts)
	assertOnlyOutputInFolder(t, opts)
	assert.Equal(t, filepath.Dir(opts.OutputPath), filepath.Dir(tempPath))
	assert.True(t, strings.HasPrefix(filepath.Base(tempPath), "."), "temporary file %q must start with a dot", tempPath)
	assert.False(t, strings.HasSuffix(tempPath, ".torrent"), "temporary file %q must not end in .torrent", tempPath)
}

func TestCreateKeepsReadOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)
	require.NoError(t, os.Chmod(opts.OutputPath, 0o444))

	_, err := CreateContext(t.Context(), opts)

	require.ErrorIs(t, err, os.ErrPermission)
	assertOldTorrent(t, opts)
	assertOnlyOutputInFolder(t, opts)
}

func TestCreateReplacesOutputSymlinkWithoutWritingTarget(t *testing.T) {
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	target := filepath.Join(t.TempDir(), "target.torrent")
	require.NoError(t, os.WriteFile(target, oldTorrentBytes, 0o644))
	if err := os.Symlink(target, opts.OutputPath); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}

	_, err := CreateContext(t.Context(), opts)
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, oldTorrentBytes, got)
	outputInfo, err := os.Lstat(opts.OutputPath)
	require.NoError(t, err)
	assert.True(t, outputInfo.Mode().IsRegular())
}
