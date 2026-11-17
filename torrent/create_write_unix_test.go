// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build !windows

package torrent

import (
	"os"
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
	setUmask(t, 0o022)
	opts := cancelCreateOptions(t, writeCancelContent(t, 64<<10))
	writeOldTorrent(t, opts)
	require.NoError(t, os.Chmod(opts.OutputPath, 0o640))

	_, err := CreateContext(t.Context(), opts)
	require.NoError(t, err)

	info, err := os.Stat(opts.OutputPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	assertOnlyOutputInFolder(t, opts)
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
