//go:build linux

package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareCreateContextFailsWhenUserFileDisappears(t *testing.T) {
	rootDir := t.TempDir()
	for i := range 200 {
		require.NoError(t, os.WriteFile(filepath.Join(rootDir, fmt.Sprintf("f%03d.bin", i)), []byte("payload"), 0o644))
	}
	victim := filepath.Join(rootDir, "zz-victim.bin")

	for i := range 20 {
		require.NoError(t, os.WriteFile(victim, []byte("payload"), 0o644))
		removed := removeAfterListing(t, rootDir, victim)

		prepared, err := prepareCreateContext(t.Context(), CreateOptions{
			Path:      rootDir,
			NoCreator: true,
			NoDate:    true,
		}, createTorrentOptions{})
		require.NoError(t, <-removed)
		if err != nil {
			require.ErrorIsf(t, err, os.ErrNotExist, "iteration %d", i)
			continue
		}
		require.Truef(t, slices.ContainsFunc(prepared.files, func(f fileEntry) bool { return f.path == victim }),
			"iteration %d left out a file that vanished after the folder was listed", i)
	}
}

func removeAfterListing(t *testing.T, dir, path string) <-chan error {
	t.Helper()
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	require.NoError(t, err)
	_, err = syscall.InotifyAddWatch(fd, dir, syscall.IN_CLOSE_NOWRITE|syscall.IN_ONLYDIR)
	require.NoError(t, err)

	removed := make(chan error, 1)
	go func() {
		defer syscall.Close(fd)
		buf := make([]byte, syscall.SizeofInotifyEvent+syscall.NAME_MAX+1)
		if _, err := syscall.Read(fd, buf); err != nil {
			removed <- err
			return
		}
		removed <- os.Remove(path)
	}()
	return removed
}
