package torrent

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

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