package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/types"
)

// setup mock functions for testing
func setupTestMocks(t *testing.T) {
	// mock create torrent function
	createMock := func(opts types.CreateTorrentOptions) (*types.Torrent, error) {
		// create a simple mock torrent with metainfo
		mi := &metainfo.MetaInfo{
			InfoBytes: []byte{},
		}

		// Use path to determine if it's a directory or file
		// Add different InfoBytes content based on path (dir vs file)
		if filepath.Base(opts.Path) == "dir1" || filepath.Base(opts.Path) == "dir1/" {
			mi.InfoBytes = []byte("directory")
		} else {
			mi.InfoBytes = []byte("file")
		}

		return &types.Torrent{
			MetaInfo: mi,
		}, nil
	}

	// mock get info function
	getInfoMock := func(t *types.Torrent) *metainfo.Info {
		// create different mock infos for single file vs directory
		if t.MetaInfo.InfoBytes == nil {
			// Prevent nil pointer dereference if MetaInfo is nil
			return &metainfo.Info{
				PieceLength: 16384,
				Name:        "test",
				Length:      1024, // single file torrent
			}
		}

		// simulate different types of torrents based on info bytes content
		if string(t.MetaInfo.InfoBytes) == "directory" {
			// multi-file torrent (directory)
			return &metainfo.Info{
				PieceLength: 16384,
				Name:        "test",
				Files: []metainfo.FileInfo{
					{Path: []string{"file1"}, Length: 512},
					{Path: []string{"file2"}, Length: 512},
				},
			}
		} else {
			// single file torrent
			return &metainfo.Info{
				PieceLength: 16384,
				Name:        "test",
				Length:      1024, // single file torrent
			}
		}
	}

	// initialize package with mock functions
	Init(createMock, getInfoMock)
}

func TestProcessBatch(t *testing.T) {
	// setup test mocks
	setupTestMocks(t)

	// create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "mkbrr-batch-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// create test files and directories
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

	// create batch config file
	configPath := filepath.Join(tmpDir, "batch.yaml")
	configContent := []byte(fmt.Sprintf(`version: 1
jobs:
  - output: %s
    path: %s
    name: "Test File 1"
    trackers:
      - udp://tracker.example.com:1337/announce
    private: true
    piece_length: 16
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

	// process batch
	results, err := ProcessBatch(configPath, true, "test-version")
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// verify results
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

		// verify torrent files were created
		if _, err := os.Stat(result.Info.Path); err != nil {
			t.Errorf("Job %d torrent file not created: %v", i, err)
		}

		// basic validation of torrent info
		if result.Info.InfoHash == "" {
			t.Errorf("Job %d missing info hash", i)
		}

		if result.Info.Size == 0 {
			t.Errorf("Job %d has zero size", i)
		}

		// check specific job details
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
	// setup test mocks
	setupTestMocks(t)

	tests := []struct {
		name        string
		config      string
		expectError bool
		createFiles bool // whether to create the files mentioned in the config
	}{
		{
			name: "invalid version",
			config: `version: 2
jobs:
  - output: test.torrent
    path: test.txt`,
			expectError: true,
			createFiles: false, // version error happens before file validation
		},
		{
			name: "missing path",
			config: `version: 1
jobs:
  - output: test.torrent`,
			expectError: true,
			createFiles: false,
		},
		{
			name: "valid config",
			config: `version: 1
jobs:
  - output: test.torrent
    path: test.txt`,
			expectError: false,
			createFiles: true, // need to create the files for this test
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// create temp dir for test
			tmpDir, err := os.MkdirTemp("", "mkbrr-batch-validation")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// create config file
			configPath := filepath.Join(tmpDir, "batch.yaml")
			if err := os.WriteFile(configPath, []byte(tc.config), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// create test files if needed
			if tc.createFiles {
				// extract path from config using simple string operations
				// this is a simplistic approach - in a real implementation you might use yaml parsing
				lines := strings.Split(tc.config, "\n")
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						continue
					}

					parts := strings.Split(line, ":")
					if len(parts) < 2 {
						continue
					}

					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					if key == "path" {
						// create the file or directory mentioned in the path
						filePath := filepath.Join(tmpDir, value)
						// create parent directories
						if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
							t.Fatalf("Failed to create directory: %v", err)
						}
						// create an empty file
						if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
							t.Fatalf("Failed to create test file: %v", err)
						}

						// update config to use the absolute path
						tc.config = strings.Replace(tc.config, "path: "+value, "path: "+filePath, 1)
					}
				}

				// rewrite the updated config file
				if err := os.WriteFile(configPath, []byte(tc.config), 0644); err != nil {
					t.Fatalf("Failed to write updated config file: %v", err)
				}
			}

			// test validation
			_, err = ProcessBatch(configPath, false, "test-version")

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
