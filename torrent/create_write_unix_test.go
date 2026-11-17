// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build !windows

package torrent

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setUmask(t *testing.T, mask int) {
	t.Helper()
	prev := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(prev) })
}

func TestCreateNewFileFollowsUmask(t *testing.T) {
	for _, tc := range []struct {
		umask int
		want  os.FileMode
	}{
		{0o022, 0o644},
		{0o002, 0o664},
	} {
		t.Run(tc.want.String(), func(t *testing.T) {
			setUmask(t, tc.umask)
			opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))

			_, err := CreateContext(t.Context(), opts)
			require.NoError(t, err)

			info, err := os.Stat(opts.OutputPath)
			require.NoError(t, err)
			assert.Equal(t, tc.want, info.Mode().Perm())
		})
	}
}

func TestCreateReplacedFileKeepsPermissionBits(t *testing.T) {
	for _, mode := range []os.FileMode{0o600, 0o664} {
		t.Run(mode.String(), func(t *testing.T) {
			setUmask(t, 0o022)
			opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
			writeOldTorrent(t, opts)
			require.NoError(t, os.Chmod(opts.OutputPath, mode))
			prev := openTempTorrentFile
			t.Cleanup(func() { openTempTorrentFile = prev })
			var openPerm os.FileMode
			openTempTorrentFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
				openPerm = perm
				return prev(name, flag, perm)
			}

			_, err := CreateContext(t.Context(), opts)
			require.NoError(t, err)

			assert.Equal(t, mode, openPerm, "the temporary file must not start wider than the old file")
			info, err := os.Stat(opts.OutputPath)
			require.NoError(t, err)
			assert.Equal(t, mode, info.Mode().Perm())
		})
	}
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
}
