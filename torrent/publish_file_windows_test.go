//go:build windows

package torrent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateInPlaceWaitsForAnotherReader(t *testing.T) {
	content := writeCancelContent(t, 32<<10)
	dir := t.TempDir()
	input := filepath.Join(dir, "release.torrent")
	_, err := Create(CreateOptions{Path: content, OutputPath: input, PieceLengthExp: new(uint(15)), NoDate: true, Quiet: true})
	require.NoError(t, err)
	before, err := os.Stat(input)
	require.NoError(t, err)
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
	after, err := os.Stat(input)
	require.NoError(t, err)
	assert.False(t, os.SameFile(before, after), "release.torrent was not replaced")
	assertFolderHolds(t, dir, "release.torrent")
}
