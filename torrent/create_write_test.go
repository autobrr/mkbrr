// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	require.NoError(t, os.WriteFile(target, oldTorrentBytes, 0o444))
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

func TestCreateMakesMissingOutputFolderBeforeHashing(t *testing.T) {
	content := writeCancelContent(t, 64<<10)
	baseInfo, err := CreateContext(t.Context(), cancelCreateOptions(t, content))
	require.NoError(t, err)

	tests := []struct {
		name  string
		setup func(t *testing.T, opts *CreateOptions) string
	}{
		{
			name: "output path with missing nested parents",
			setup: func(t *testing.T, opts *CreateOptions) string {
				opts.OutputPath = filepath.Join(t.TempDir(), "missing", "nested", "out.torrent")
				return opts.OutputPath
			},
		},
		{
			name: "relative output path with missing output dir",
			setup: func(t *testing.T, opts *CreateOptions) string {
				opts.OutputDir = filepath.Join(t.TempDir(), "missing-out")
				opts.OutputPath = filepath.Join("sub", "custom")
				return filepath.Join(opts.OutputDir, "sub", "custom.torrent")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := cancelCreateOptions(t, content)
			wantPath := tt.setup(t, &opts)
			folder := filepath.Dir(wantPath)

			var (
				once             sync.Once
				sawCallback      bool
				folderExisted    bool
				entriesAtHashing int
			)
			opts.ProgressCallback = func(_, _ int, _ float64) {
				once.Do(func() {
					sawCallback = true
					if info, statErr := os.Stat(folder); statErr == nil && info.IsDir() {
						folderExisted = true
						if entries, readErr := os.ReadDir(folder); readErr == nil {
							entriesAtHashing = len(entries)
						}
					}
				})
			}

			info, err := CreateContext(t.Context(), opts)
			require.NoError(t, err)
			assert.True(t, sawCallback)
			assert.True(t, folderExisted, "output folder must exist before hashing starts")
			assert.Zero(t, entriesAtHashing, "test write must not leave a temporary file in the folder")
			assert.Equal(t, wantPath, info.Path)
			assert.Equal(t, baseInfo.InfoHash, info.InfoHash)
			assertOnlyOutputInFolder(t, CreateOptions{OutputPath: wantPath})
		})
	}
}

func TestCreateRejectsUnwritableFolderBeforeHashing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("folder permission bits do not block writes on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only folder")
	}

	content := writeCancelContent(t, 64<<10)
	roDir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.MkdirAll(roDir, 0o755))
	require.NoError(t, os.Chmod(roDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	var rec progressRecorder
	opts := cancelCreateOptions(t, content)
	opts.OutputPath = filepath.Join(roDir, "out.torrent")
	opts.ProgressCallback = rec.callback

	_, err := CreateContext(t.Context(), opts)

	require.ErrorIs(t, err, os.ErrPermission)
	require.ErrorContains(t, err, roDir)
	assert.Empty(t, rec.snapshot(), "create must stop before any progress callback")
	entries, readErr := os.ReadDir(roDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestCreateLeavesNoTempFileAfterTestWrite(t *testing.T) {
	content := writeCancelContent(t, 256<<10)

	t.Run("successful create", func(t *testing.T) {
		prev := openTempTorrentFile
		t.Cleanup(func() { openTempTorrentFile = prev })
		var tempPaths []string
		openTempTorrentFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			tempPaths = append(tempPaths, name)
			return prev(name, flag, perm)
		}

		opts := cancelCreateOptions(t, content)
		_, err := CreateContext(t.Context(), opts)
		require.NoError(t, err)

		require.Len(t, tempPaths, 2)
		for _, tempPath := range tempPaths {
			assert.Equal(t, filepath.Dir(opts.OutputPath), filepath.Dir(tempPath))
			assert.True(t, strings.HasPrefix(filepath.Base(tempPath), "."), "temporary file %q must start with a dot", tempPath)
			assert.False(t, strings.HasSuffix(tempPath, ".torrent"), "temporary file %q must not end in .torrent", tempPath)
		}
		assertOnlyOutputInFolder(t, opts)
	})

	t.Run("canceled create", func(t *testing.T) {
		ctx, rec := cancelOnLastPiece(t)
		opts := cancelCreateOptions(t, content)
		opts.OutputPath = filepath.Join(t.TempDir(), "new-folder", "out.torrent")
		opts.ProgressCallback = rec.callback

		_, err := CreateContext(ctx, opts)
		rec.finish(t)

		require.ErrorIs(t, err, context.Canceled)
		entries, readErr := os.ReadDir(filepath.Dir(opts.OutputPath))
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})
}
