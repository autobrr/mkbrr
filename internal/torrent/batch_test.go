package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessBatch(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "mkbrr-batch-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files and directories
	testFiles := []struct {
		path    string
		content string
	}{
		{
			path:    "file1.txt",
			content: "test file 1 content",
		},
		{
			path:    "dir1/file2.txt",
			content: "test file 2 content",
		},
		{
			path:    "dir1/file3.txt",
			content: "test file 3 content",
		},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create batch config file
	configPath := filepath.Join(tmpDir, "batch.yaml")
	configContent := []byte(fmt.Sprintf(`version: 1
jobs:
  - output: %s
    path: %s
    name: "Test File 1"
    trackers:
      - udp://tracker.example.com:1337/announce
    private: true
    piece_length: 14
  - output: %s
    path: %s
    name: "Test Directory"
    trackers:
      - udp://tracker.example.com:1337/announce
    webseeds:
      - https://example.com/files/
    comment: "Test batch torrent"
`,
		filepath.Join(tmpDir, "file1.torrent"),
		filepath.Join(tmpDir, "file1.txt"),
		filepath.Join(tmpDir, "dir1.torrent"),
		filepath.Join(tmpDir, "dir1")))

	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Process batch
	results, err := ProcessBatch(configPath, true, "test-version")
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("Job %d failed: %v", i, result.Error)
			continue
		}

		if result.Info == nil {
			t.Errorf("Job %d missing info", i)
			continue
		}

		// Verify torrent files were created
		if _, err := os.Stat(result.Info.Path); err != nil {
			t.Errorf("Job %d torrent file not created: %v", i, err)
		}

		// Basic validation of torrent info
		if result.Info.InfoHash == "" {
			t.Errorf("Job %d missing info hash", i)
		}

		if result.Info.Size == 0 {
			t.Errorf("Job %d has zero size", i)
		}

		// Check specific job details
		switch i {
		case 0: // file1.txt
			if result.Info.Files != 0 {
				t.Errorf("Expected single file torrent, got %d files", result.Info.Files)
			}
		case 1: // dir1
			if result.Info.Files != 2 {
				t.Errorf("Expected 2 files in directory torrent, got %d", result.Info.Files)
			}
		}
	}
}

func TestBatchValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "invalid version",
			config: `version: 2
jobs:
  - output: test.torrent
    path: test.txt`,
			expectError: true,
		},
		{
			name: "missing path",
			config: `version: 1
jobs:
  - output: test.torrent`,
			expectError: true,
		},
		{
			name: "missing output",
			config: `version: 1
jobs:
  - path: test.txt`,
			expectError: true,
		},
		{
			name: "invalid piece length",
			config: `version: 1
jobs:
  - output: test.torrent
    path: test.txt
    piece_length: 25`,
			expectError: true,
		},
		{
			name: "empty jobs",
			config: `version: 1
jobs: []`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "mkbrr-batch-validation")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configPath := filepath.Join(tmpDir, "batch.yaml")
			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = ProcessBatch(configPath, false, "test-version")
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
