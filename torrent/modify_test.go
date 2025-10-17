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

func TestModify_NameArgument(t *testing.T) {

	tracker := "https://unknown.customtracker.com/announce"
	tracker2 := "https://unknown.customtracker2.com/announce"

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "mkbrr-modify-TestModify_NameArgument-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	filename := "oldname"
	testFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(testFile, []byte("modify test with -name argument"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test torrent
	createresult, err := Create(CreateOptions{
		Path:        testFile,
		Name:        "oldname",
		OutputDir:   tmpDir,
		TrackerURLs: []string{tracker},
		SkipPrefix:  true,
		Quiet:       false,
		Verbose:     true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify the file was actually created
	torrentFilepath := createresult.Path
	if _, err := os.Stat(torrentFilepath); err != nil {
		t.Fatalf("Created torrent file, %q does not exist: %v", torrentFilepath, err)
	}

	// Test cases
	tests := []struct {
		name             string
		path             string
		opts             ModifyOptions
		expectedName     string
		expectedFilename string
	}{
		{
			name: "No --name argument no --skip-prefix no -o",
			path: torrentFilepath,
			opts: ModifyOptions{
				SkipPrefix: false,
				Quiet:      true,
			},
			expectedName:     "oldname",
			expectedFilename: "modified_oldname.torrent",
		},
		{
			name: "No --name argument --skip-prefix present -o supplied",
			path: torrentFilepath,
			opts: ModifyOptions{
				OutputPattern: "customfilename",
				SkipPrefix:    true,
				Quiet:         true,
			},
			expectedName:     "oldname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "No --name argument no --skip-prefix -o supplied -t supplied",
			path: torrentFilepath,
			opts: ModifyOptions{
				OutputPattern: "customfilename",
				TrackerURLs:   []string{tracker2},
				SkipPrefix:    false,
				Quiet:         true,
			},
			expectedName:     "oldname",
			expectedFilename: "customfilename.torrent", // original behavior -  does not add prefix on modify
		},
		{
			name: "With --name argument no --skip-prefix no -o",
			path: torrentFilepath,
			opts: ModifyOptions{
				Name:       "customname",
				SkipPrefix: false,
				Quiet:      true,
			},
			expectedName:     "customname",
			expectedFilename: "modified_oldname.torrent",
		},
		{
			name: "With --name argument --skip-prefix present -o supplied",
			path: torrentFilepath,
			opts: ModifyOptions{
				Name:          "customname",
				OutputPattern: "customfilename",
				SkipPrefix:    true,
				Quiet:         true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "With --name argument no --skip-prefix -o supplied",
			path: torrentFilepath,
			opts: ModifyOptions{
				Name:          "customname",
				OutputPattern: "customfilename",
				SkipPrefix:    false,
				Quiet:         true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent", // original behavior -  does not add prefix on modify
		},
		{
			name: "With --name argument no --skip-prefix -o supplied -t supplied",
			path: torrentFilepath,
			opts: ModifyOptions{
				Name:          "customname",
				OutputPattern: "customfilename",
				TrackerURLs:   []string{tracker2},
				SkipPrefix:    false,
				Quiet:         true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent", // original behavior -  does not add prefix on modify
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Modify the torrent
			result, err := ModifyTorrent(tt.path, tt.opts)
			if err != nil {
				t.Fatalf("Modify() failed: %v", err)
			}

			// Verify the file was actually created or modified in place
			if _, err := os.Stat(result.Path); err != nil {
				t.Fatalf("Modified torrent file, %q does not exist: %v", tt.path, err)
			}

			// Get the modified torrent internals
			mi, err := LoadFromFile(result.OutputPath)
			if err != nil {
				t.Fatalf("Failed to load modified torrent: %v", err)
			}
			info, err := mi.UnmarshalInfo()
			if err != nil {
				t.Fatalf("Failed to unmarshal info from created torrent: %v", err)
			}

			// Check the name
			if info.Name != tt.expectedName {
				t.Fatalf("Expected torrent name %q, got %q", tt.expectedName, info.Name)
			}

			// Check the output filename
			createdFilename := filepath.Base(result.OutputPath)
			if createdFilename != tt.expectedFilename {
				t.Fatalf("Expected output filename %q, got %q", tt.expectedFilename, createdFilename)
			}

			t.Logf("Torrent modified with name %q and filename %q as expected.", info.Name, createdFilename)
		})
	}
}
