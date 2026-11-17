// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"context"
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
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{filepath.Base(opts.OutputPath)}, names)
}

func failPublish(t *testing.T, publishErr error) *string {
	t.Helper()
	prev := publishCreatedTorrent
	t.Cleanup(func() { publishCreatedTorrent = prev })
	var tempPath string
	publishCreatedTorrent = func(temp, _ string, _ bool) error {
		tempPath = temp
		return publishErr
	}
	return &tempPath
}

func TestCreateReplacesExistingFile(t *testing.T) {
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)
	want, err := CreateTorrent(opts)
	require.NoError(t, err)

	info, err := CreateContext(t.Context(), opts)
	require.NoError(t, err)

	assert.Equal(t, want.HashInfoBytes().String(), info.InfoHash)
	got, err := os.ReadFile(opts.OutputPath)
	require.NoError(t, err)
	assert.NotEqual(t, oldTorrentBytes, got)
	assertOnlyOutputInFolder(t, opts)
}

func TestCreateKeepsExistingFileWhenStopped(t *testing.T) {
	ctx, rec := cancelOnLastPiece(t)
	opts := cancelCreateOptions(t, writeCancelContent(t, 256<<10))
	opts.ProgressCallback = rec.callback
	writeOldTorrent(t, opts)

	_, err := CreateContext(ctx, opts)
	rec.finish(t)

	require.ErrorIs(t, err, context.Canceled)
	assertOldTorrent(t, opts)
	assertOnlyOutputInFolder(t, opts)
}

func TestCreateKeepsExistingFileWhenPublishFails(t *testing.T) {
	publishErr := errors.New("disk full")
	tempPath := failPublish(t, publishErr)
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)

	_, err := CreateContext(t.Context(), opts)

	require.ErrorIs(t, err, publishErr)
	assertOldTorrent(t, opts)
	assertOnlyOutputInFolder(t, opts)
	assert.Equal(t, filepath.Dir(opts.OutputPath), filepath.Dir(*tempPath))
	assert.True(t, strings.HasPrefix(filepath.Base(*tempPath), "."), "temporary file %q must start with a dot", *tempPath)
	assert.False(t, strings.HasSuffix(*tempPath, ".torrent"), "temporary file %q must not end in .torrent", *tempPath)
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
	_, err = VerifyData(VerifyOptions{TorrentPath: opts.OutputPath, ContentPath: opts.Path, Quiet: true})
	require.NoError(t, err)
}

func TestCreateWritesTemporaryFileInFolderThatTheKernelResolves(t *testing.T) {
	realDir := filepath.Join(t.TempDir(), "real", "sub")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	linkParent := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(linkParent, "link")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	opts.OutputPath = filepath.Join(linkParent, "link") + string(filepath.Separator) + ".." + string(filepath.Separator) + "out.torrent"
	wantDir := filepath.Dir(realDir)

	prev := publishCreatedTorrent
	t.Cleanup(func() { publishCreatedTorrent = prev })
	var tempInWantDir bool
	publishCreatedTorrent = func(temp, out string, replace bool) error {
		_, err := os.Stat(filepath.Join(wantDir, filepath.Base(temp)))
		tempInWantDir = err == nil
		return prev(temp, out, replace)
	}

	_, err := CreateContext(t.Context(), opts)
	require.NoError(t, err)

	assert.True(t, tempInWantDir, "temporary file must be in %s", wantDir)
	assert.FileExists(t, filepath.Join(wantDir, "out.torrent"))
}
