package torrent

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/stretchr/testify/assert"
)

func TestShowFiles_WithSubdirectories(t *testing.T) {
	tests := []struct {
		name     string
		files    []fileEntry
		expected []string // Lines that should appear in output
	}{
		{
			name: "Files with Screens subdirectory",
			files: []fileEntry{
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0001.png"), length: 1024 * 1024},
				{path: filepath.Join("/test", "Screens", "ShowName.S01E10.720p.x264-Group.Screen0002.png"), length: 1024 * 1024},
				{path: filepath.Join("/test", "ShowName.S01E10.720p.x264-Group.mkv"), length: 500 * 1024 * 1024},
				{path: filepath.Join("/test", "ShowName.S01E10.720p.x264-Group.nfo"), length: 5 * 1024},
			},
			expected: []string{
				"└─ test",
				"  ├─ Screens",
				"  │ ├─ ShowName.S01E10.720p.x264-Group.Screen0001.png",
				"  │ └─ ShowName.S01E10.720p.x264-Group.Screen0002.png",
				"  ├─ ShowName.S01E10.720p.x264-Group.mkv",
				"  └─ ShowName.S01E10.720p.x264-Group.nfo",
			},
		},
		{
			name: "Multiple subdirectories",
			files: []fileEntry{
				{path: filepath.Join("/media", "Season 01", "Show.S01E01.mkv"), length: 400 * 1024 * 1024},
				{path: filepath.Join("/media", "Season 01", "Show.S01E02.mkv"), length: 450 * 1024 * 1024},
				{path: filepath.Join("/media", "Season 02", "Show.S02E01.mkv"), length: 420 * 1024 * 1024},
				{path: filepath.Join("/media", "info.txt"), length: 1024},
			},
			expected: []string{
				"└─ media",
				"  ├─ Season 01",
				"  │ ├─ Show.S01E01.mkv",
				"  │ └─ Show.S01E02.mkv",
				"  ├─ Season 02",
				"  │ └─ Show.S02E01.mkv",
				"  └─ info.txt",
			},
		},
		{
			name: "Flat directory structure",
			files: []fileEntry{
				{path: filepath.Join("/downloads", "file1.mkv"), length: 100 * 1024 * 1024},
				{path: filepath.Join("/downloads", "file2.mkv"), length: 200 * 1024 * 1024},
				{path: filepath.Join("/downloads", "file3.nfo"), length: 2048},
			},
			expected: []string{
				"└─ downloads",
				"  ├─ file1.mkv",
				"  ├─ file2.mkv",
				"  └─ file3.nfo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewFormatter(false)
			display := NewDisplay(formatter)
			display.output = &buf

			display.ShowFiles(tc.files, 4)

			output := buf.String()

			// Check that each expected line appears in the output
			for _, expectedLine := range tc.expected {
				// Remove ANSI color codes for comparison
				cleanOutput := stripAnsiCodes(output)
				assert.Contains(t, cleanOutput, expectedLine,
					"Output should contain line: %s", expectedLine)
			}
		})
	}
}

func TestShowFiles_EmptyFiles(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewFormatter(false)
	display := NewDisplay(formatter)
	display.output = &buf

	display.ShowFiles([]fileEntry{}, 4)

	output := buf.String()
	assert.Contains(t, output, "Using 4 worker(s)")
	assert.Contains(t, output, "Files being hashed:")
}

func TestShowFiles_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewFormatter(false)
	display := NewDisplay(formatter)
	display.SetQuiet(true)
	display.output = &buf

	files := []fileEntry{
		{path: filepath.Join("/test", "file.mkv"), length: 100 * 1024 * 1024},
	}

	display.ShowFiles(files, 4)

	output := buf.String()
	assert.Empty(t, output, "No output should be produced in quiet mode")
}

// Helper function to create a properly initialized torrent with InfoBytes
func createTestTorrent(metaInfo *metainfo.MetaInfo, info *metainfo.Info) (*Torrent, error) {
	// Marshal the info to get InfoBytes
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		return nil, err
	}
	metaInfo.InfoBytes = infoBytes
	return &Torrent{MetaInfo: metaInfo}, nil
}

func TestShowTorrentInfo_Complete(t *testing.T) {
	tests := []struct {
		name     string
		torrent  *Torrent
		info     *metainfo.Info
		expected []string // Lines that should appear in output
	}{
		{
			name: "Complete torrent with all fields",
			torrent: func() *Torrent {
				metaInfo := &metainfo.MetaInfo{
					Announce:     "http://tracker.example.com/announce",
					AnnounceList: [][]string{{"http://tracker1.example.com/announce", "http://tracker2.example.com/announce"}},
					Comment:      "Test torrent comment",
					CreatedBy:    "mkbrr/1.0.0",
					CreationDate: 1640995200, // 2022-01-01 00:00:00 UTC
					UrlList:      []string{"http://seed1.example.com/", "http://seed2.example.com/"},
				}
				info := &metainfo.Info{
					Name:        "Test Torrent",
					PieceLength: 262144,              // 256KB
					Pieces:      make([]byte, 20*10), // 10 pieces
					Private:     &[]bool{true}[0],
					Source:      "test-source",
					Files: []metainfo.FileInfo{
						{Path: []string{"file1.txt"}, Length: 1024},
						{Path: []string{"subdir", "file2.txt"}, Length: 2048},
					},
				}
				torrent, _ := createTestTorrent(metaInfo, info)
				return torrent
			}(),
			info: &metainfo.Info{
				Name:        "Test Torrent",
				PieceLength: 262144,              // 256KB
				Pieces:      make([]byte, 20*10), // 10 pieces
				Private:     &[]bool{true}[0],
				Source:      "test-source",
				Files: []metainfo.FileInfo{
					{Path: []string{"file1.txt"}, Length: 1024},
					{Path: []string{"subdir", "file2.txt"}, Length: 2048},
				},
			},
			expected: []string{
				"Torrent info:",
				"Name:         Test Torrent",
				"Hash:",
				"Size:         3.0 KiB",
				"Piece length: 256 KiB",
				"Pieces:       10",
				"Magnet:",
				"Trackers:",
				"  http://tracker1.example.com/announce",
				"  http://tracker2.example.com/announce",
				"Web seeds:",
				"  http://seed1.example.com/",
				"  http://seed2.example.com/",
				"Private:      yes",
				"Source:       test-source",
				"Comment:      Test torrent comment",
				"Created by:   mkbrr/1.0.0",
				"Created on:   2022-01-01",
				"Files:        2",
			},
		},
		{
			name: "Minimal torrent with single tracker",
			torrent: func() *Torrent {
				metaInfo := &metainfo.MetaInfo{
					Announce: "http://tracker.example.com/announce",
				}
				info := &metainfo.Info{
					Name:        "Simple Torrent",
					PieceLength: 1048576,            // 1MB
					Pieces:      make([]byte, 20*5), // 5 pieces
				}
				torrent, _ := createTestTorrent(metaInfo, info)
				return torrent
			}(),
			info: &metainfo.Info{
				Name:        "Simple Torrent",
				PieceLength: 1048576,            // 1MB
				Pieces:      make([]byte, 20*5), // 5 pieces
			},
			expected: []string{
				"Torrent info:",
				"Name:         Simple Torrent",
				"Hash:",
				"Size:         0 B",
				"Piece length: 1.0 MiB",
				"Pieces:       5",
				"Magnet:",
				"Tracker:      http://tracker.example.com/announce",
			},
		},
		{
			name: "Single file torrent",
			torrent: func() *Torrent {
				metaInfo := &metainfo.MetaInfo{
					Announce: "http://tracker.example.com/announce",
					Comment:  "Single file torrent",
				}
				info := &metainfo.Info{
					Name:        "single-file.txt",
					PieceLength: 524288,             // 512KB
					Pieces:      make([]byte, 20*2), // 2 pieces
					Length:      1024000,            // ~1MB
				}
				torrent, _ := createTestTorrent(metaInfo, info)
				return torrent
			}(),
			info: &metainfo.Info{
				Name:        "single-file.txt",
				PieceLength: 524288,             // 512KB
				Pieces:      make([]byte, 20*2), // 2 pieces
				Length:      1024000,            // ~1MB
			},
			expected: []string{
				"Torrent info:",
				"Name:         single-file.txt",
				"Hash:",
				"Size:         1000 KiB",
				"Piece length: 512 KiB",
				"Pieces:       2",
				"Magnet:",
				"Tracker:      http://tracker.example.com/announce",
				"Comment:      Single file torrent",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewFormatter(false)
			display := NewDisplay(formatter)
			display.output = &buf

			display.ShowTorrentInfo(tc.torrent, tc.info)

			output := buf.String()
			cleanOutput := stripAnsiCodes(output)

			// Check that each expected line appears in the output
			for _, expectedLine := range tc.expected {
				assert.Contains(t, cleanOutput, expectedLine,
					"Output should contain line: %s", expectedLine)
			}

			// Verify that "Torrent info:" header is present
			assert.Contains(t, cleanOutput, "Torrent info:")

			// Verify that magnet link is displayed (since we assume it always succeeds)
			assert.Contains(t, cleanOutput, "Magnet:")
		})
	}
}

func TestShowTorrentInfo_EmptyFields(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewFormatter(false)
	display := NewDisplay(formatter)
	display.output = &buf

	// Torrent with minimal fields
	metaInfo := &metainfo.MetaInfo{}
	info := &metainfo.Info{
		Name:        "Minimal Torrent",
		PieceLength: 262144,
		Pieces:      make([]byte, 20*1),
	}
	torrent, _ := createTestTorrent(metaInfo, info)

	display.ShowTorrentInfo(torrent, info)

	output := buf.String()
	cleanOutput := stripAnsiCodes(output)

	// Should still show basic info
	assert.Contains(t, cleanOutput, "Torrent info:")
	assert.Contains(t, cleanOutput, "Name:         Minimal Torrent")
	assert.Contains(t, cleanOutput, "Magnet:")

	// Should not show empty fields
	assert.NotContains(t, cleanOutput, "Tracker:")
	assert.NotContains(t, cleanOutput, "Trackers:")
	assert.NotContains(t, cleanOutput, "Web seeds:")
	assert.NotContains(t, cleanOutput, "Private:")
	assert.NotContains(t, cleanOutput, "Source:")
	assert.NotContains(t, cleanOutput, "Comment:")
	assert.NotContains(t, cleanOutput, "Created by:")
	assert.NotContains(t, cleanOutput, "Created on:")
	assert.NotContains(t, cleanOutput, "Files:")
}

func TestShowTorrentInfo_PrivateField(t *testing.T) {
	tests := []struct {
		name     string
		private  *bool
		expected bool
	}{
		{
			name:     "Private torrent",
			private:  &[]bool{true}[0],
			expected: true,
		},
		{
			name:     "Public torrent",
			private:  &[]bool{false}[0],
			expected: false,
		},
		{
			name:     "Private field nil",
			private:  nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewFormatter(false)
			display := NewDisplay(formatter)
			display.output = &buf

			metaInfo := &metainfo.MetaInfo{}
			info := &metainfo.Info{
				Name:        "Test Torrent",
				PieceLength: 262144,
				Pieces:      make([]byte, 20*1),
				Private:     tc.private,
			}
			torrent, _ := createTestTorrent(metaInfo, info)

			display.ShowTorrentInfo(torrent, info)

			output := buf.String()
			cleanOutput := stripAnsiCodes(output)

			if tc.expected {
				assert.Contains(t, cleanOutput, "Private:      yes")
			} else {
				assert.NotContains(t, cleanOutput, "Private:")
			}
		})
	}
}

// Helper function to strip ANSI color codes from output
func stripAnsiCodes(s string) string {
	// Simple regex pattern to remove ANSI escape sequences
	for {
		start := strings.Index(s, "\x1b[")
		if start == -1 {
			break
		}
		end := strings.IndexByte(s[start:], 'm')
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return s
}
