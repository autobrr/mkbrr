package torrent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModifyTorrent_OutputDirPriority(t *testing.T) {
	// Setup temporary directories for test
	tmpDir, err := os.MkdirTemp("", "mkbrr-modify-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a non-empty file in the temp directory for the torrent content
	dummyFilePath := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	// Create test torrent file (minimal content for test)
	torrentPath := filepath.Join(tmpDir, "test.torrent")
	torrent, err := Create(CreateOptions{
		Path:       tmpDir,
		OutputPath: torrentPath,
		IsPrivate:  true,
		NoDate:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create test torrent: %v", err)
	}

	// Create preset file
	presetDir := filepath.Join(tmpDir, "presets")
	if err := os.Mkdir(presetDir, 0755); err != nil {
		t.Fatalf("Failed to create presets dir: %v", err)
	}
	presetPath := filepath.Join(presetDir, "presets.yaml")
	presetConfig := `version: 1
presets:
  test:
    output_dir: "` + filepath.ToSlash(filepath.Join(tmpDir, "preset_output")) + `"
    private: true
    source: "TEST"
`
	if err := os.WriteFile(presetPath, []byte(presetConfig), 0644); err != nil {
		t.Fatalf("Failed to write preset config: %v", err)
	}

	// Create the output directories
	cmdLineOutputDir := filepath.Join(tmpDir, "cmdline_output")
	presetOutputDir := filepath.Join(tmpDir, "preset_output")
	if err := os.Mkdir(cmdLineOutputDir, 0755); err != nil {
		t.Fatalf("Failed to create cmdline output dir: %v", err)
	}
	if err := os.Mkdir(presetOutputDir, 0755); err != nil {
		t.Fatalf("Failed to create preset output dir: %v", err)
	}

	// Test cases
	tests := []struct {
		name           string
		opts           ModifyOptions
		expectedOutDir string
	}{
		{
			name: "Command-line OutputDir should take precedence",
			opts: ModifyOptions{
				PresetName: "test",
				PresetFile: presetPath,
				OutputDir:  cmdLineOutputDir,
				Version:    "test",
			},
			expectedOutDir: cmdLineOutputDir,
		},
		{
			name: "Preset OutputDir should be used when no command-line OutputDir",
			opts: ModifyOptions{
				PresetName: "test",
				PresetFile: presetPath,
				OutputDir:  "", // empty to use preset
				Version:    "test",
			},
			expectedOutDir: presetOutputDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ModifyTorrent(torrent.Path, tt.opts)
			if err != nil {
				t.Fatalf("ModifyTorrent failed: %v", err)
			}

			// Verify the output path contains the expected directory
			dir := filepath.Dir(result.OutputPath)
			if dir != tt.expectedOutDir {
				t.Errorf("Expected output directory %q, got %q", tt.expectedOutDir, dir)
			}

			// Verify the file was actually created in the expected directory
			if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
				t.Errorf("Output file wasn't created at expected path: %s", result.OutputPath)
			}
		})
	}
}

func TestModifyTorrent_MultipleAndNoTrackers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mkbrr-modify-multitracker-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dummyFilePath := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	torrentPath := filepath.Join(tmpDir, "test.torrent")
	torrent, err := Create(CreateOptions{
		Path:       tmpDir,
		OutputPath: torrentPath,
		IsPrivate:  true,
		NoDate:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create test torrent: %v", err)
	}

	t.Run("Multiple trackers", func(t *testing.T) {
		opts := ModifyOptions{
			OutputDir: tmpDir,
			TrackerURLs: []string{
				"https://tracker1.com/announce",
				"https://tracker2.com/announce",
				"https://tracker3.com/announce",
			},
			Version: "test",
		}
		result, err := ModifyTorrent(torrent.Path, opts)
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if result.OutputPath == "" {
			t.Errorf("Expected output path to be set")
		}
		mi, err := LoadFromFile(result.OutputPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		if mi.Announce != opts.TrackerURLs[0] {
			t.Errorf("Announce not set to first tracker, got %q", mi.Announce)
		}
		if mi.AnnounceList == nil || len(mi.AnnounceList) != 1 || len(mi.AnnounceList[0]) != 3 {
			t.Errorf("AnnounceList not set correctly: %#v", mi.AnnounceList)
		}
		for i, tracker := range opts.TrackerURLs {
			if mi.AnnounceList[0][i] != tracker {
				t.Errorf("AnnounceList[%d] = %q, want %q", i, mi.AnnounceList[0][i], tracker)
			}
		}
	})

	t.Run("No tracker", func(t *testing.T) {
		opts := ModifyOptions{
			OutputDir:   tmpDir,
			TrackerURLs: nil,
			Version:     "test",
		}
		result, err := ModifyTorrent(torrent.Path, opts)
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if result.OutputPath == "" {
			t.Errorf("Expected output path to be set")
		}
		mi, err := LoadFromFile(result.OutputPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		if mi.Announce != "" {
			t.Errorf("Announce should be empty when no tracker, got %q", mi.Announce)
		}
		if len(mi.AnnounceList) > 0 && len(mi.AnnounceList[0]) > 0 {
			t.Errorf("AnnounceList should be empty or nil when no tracker, got %#v", mi.AnnounceList)
		}
	})
}
