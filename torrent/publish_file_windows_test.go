//go:build windows

package torrent

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestUpdateInPlaceWaitsForAnotherReader(t *testing.T) {
	content := writeCancelContent(t, 32<<10)
	dir := t.TempDir()
	input := filepath.Join(dir, "release.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: input, PieceLengthExp: new(uint(15)), NoDate: true, Quiet: true})
	require.NoError(t, err)
	before := statByHandle(t, input)
	prev := publishTorrentOutput
	t.Cleanup(func() { publishTorrentOutput = prev })
	publishTorrentOutput = func(tempPath, outputPath string, replace bool) error {
		reader, err := os.Open(outputPath)
		if err != nil {
			return err
		}
		time.AfterFunc(100*time.Millisecond, func() { _ = reader.Close() })
		return publishTorrentFile(tempPath, outputPath, replace)
	}

	_, err = UpdateTorrent(UpdateOptions{TorrentPath: input, ContentPath: content, InPlace: true, Quiet: true})

	require.NoError(t, err)
	assert.False(t, os.SameFile(before, statByHandle(t, input)), "release.torrent was not replaced")
	assertFolderHolds(t, dir, "release.torrent")
}

func statByHandle(t *testing.T, path string) os.FileInfo {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	info, err := f.Stat()
	require.NoError(t, err)
	return info
}

func TestWriteTorrentOutputWaitsForReaderThatDeniesWrites(t *testing.T) {
	output := filepath.Join(t.TempDir(), "release.torrent")
	require.NoError(t, os.WriteFile(output, []byte("old"), 0o644))
	name, err := windows.UTF16PtrFromString(output)
	require.NoError(t, err)
	reader, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	require.NoError(t, err)
	release := time.AfterFunc(100*time.Millisecond, func() { _ = windows.CloseHandle(reader) })
	t.Cleanup(func() {
		if release.Stop() {
			_ = windows.CloseHandle(reader)
		}
	})

	err = writeTorrentOutput(output, 0o644, true, func(w io.Writer) error {
		_, err := w.Write([]byte("new"))
		return err
	})

	require.NoError(t, err)
	got, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}
