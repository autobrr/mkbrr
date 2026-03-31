package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent/bencode"
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
		if mi.AnnounceList == nil || len(mi.AnnounceList) != len(opts.TrackerURLs) {
			t.Errorf("AnnounceList not set correctly: %#v", mi.AnnounceList)
		}
		for i, tracker := range opts.TrackerURLs {
			if len(mi.AnnounceList) <= i || len(mi.AnnounceList[i]) != 1 {
				t.Errorf("AnnounceList tier %d invalid: %#v", i, mi.AnnounceList)
				continue
			}
			if mi.AnnounceList[i][0] != tracker {
				t.Errorf("AnnounceList tier %d = %q, want %q", i, mi.AnnounceList[i][0], tracker)
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
		if len(mi.AnnounceList) > 0 {
			t.Errorf("AnnounceList should be empty or nil when no tracker, got %#v", mi.AnnounceList)
		}
	})
}

func TestModifyTorrent_PresetEntropy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mkbrr-modify-entropy-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dummyFilePath := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("test content for entropy"), 0644); err != nil {
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

	// Create preset file with entropy: true
	presetDir := filepath.Join(tmpDir, "presets")
	if err := os.Mkdir(presetDir, 0755); err != nil {
		t.Fatalf("Failed to create presets dir: %v", err)
	}
	presetPath := filepath.Join(presetDir, "presets.yaml")
	presetConfig := `version: 1
presets:
  entropy_test:
    private: true
    source: "TEST"
    entropy: true
`
	if err := os.WriteFile(presetPath, []byte(presetConfig), 0644); err != nil {
		t.Fatalf("Failed to write preset config: %v", err)
	}

	opts := ModifyOptions{
		PresetName: "entropy_test",
		PresetFile: presetPath,
		OutputDir:  tmpDir,
		Version:    "test",
	}

	result, err := ModifyTorrent(torrent.Path, opts)
	if err != nil {
		t.Fatalf("ModifyTorrent failed: %v", err)
	}

	if !result.WasModified {
		t.Fatal("Expected torrent to be modified")
	}

	// Load modified torrent and check for entropy field in info dict
	mi, err := LoadFromFile(result.OutputPath)
	if err != nil {
		t.Fatalf("Failed to load modified torrent: %v", err)
	}

	infoMap := make(map[string]interface{})
	if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
		t.Fatalf("Failed to unmarshal info bytes: %v", err)
	}

	if _, ok := infoMap["entropy"]; !ok {
		t.Error("Expected entropy field in info dict when preset has entropy: true")
	}
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
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	prefixedCreateResult, err := Create(CreateOptions{
		Path:        testFile,
		Name:        "oldname",
		OutputDir:   tmpDir,
		TrackerURLs: []string{tracker},
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("Create() with prefix failed: %v", err)
	}

	// Verify the file was actually created
	torrentFilepath := createresult.Path
	if _, err := os.Stat(torrentFilepath); err != nil {
		t.Fatalf("Created torrent file, %q does not exist: %v", torrentFilepath, err)
	}
	prefixedTorrentFilepath := prefixedCreateResult.Path
	if _, err := os.Stat(prefixedTorrentFilepath); err != nil {
		t.Fatalf("Created prefixed torrent file, %q does not exist: %v", prefixedTorrentFilepath, err)
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
		{
			name: "Prefixed input no --name argument no --skip-prefix no -o",
			path: prefixedTorrentFilepath,
			opts: ModifyOptions{
				SkipPrefix: false,
				Quiet:      true,
			},
			expectedName:     "oldname",
			expectedFilename: "modified_oldname.torrent",
		},
		{
			name: "Prefixed input with --name argument no --skip-prefix no -o",
			path: prefixedTorrentFilepath,
			opts: ModifyOptions{
				Name:       "customname",
				SkipPrefix: false,
				Quiet:      true,
			},
			expectedName:     "customname",
			expectedFilename: "modified_oldname.torrent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Modify the torrent
			result, err := ModifyTorrent(tt.path, tt.opts)
			if err != nil {
				t.Fatalf("Modify() failed: %v", err)
			}

			// Verify the output file was actually written
			if _, err := os.Stat(result.OutputPath); err != nil {
				t.Fatalf("Modified torrent output file, %q does not exist: %v", result.OutputPath, err)
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

func TestModifyTorrent_RemoveFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mkbrr-modify-remove-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// create a dummy file for torrent content
	dummyFilePath := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("test content for removal"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
	}

	// create a test torrent with private, source, and comment set
	torrentPath := filepath.Join(tmpDir, "test.torrent")
	_, err = Create(CreateOptions{
		Path:       tmpDir,
		OutputPath: torrentPath,
		IsPrivate:  true,
		Source:     "TESTSOURCE",
		Comment:    "test comment",
		NoDate:     true,
	})
	if err != nil {
		t.Fatalf("Failed to create test torrent: %v", err)
	}

	t.Run("RemovePrivate", func(t *testing.T) {
		outPath := filepath.Join(tmpDir, "removed_private.torrent")
		result, err := ModifyTorrent(torrentPath, ModifyOptions{
			RemovePrivate: true,
			OutputDir:     tmpDir,
			OutputPattern: "removed_private",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified")
		}

		// verify private field is removed
		mi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		info, err := mi.UnmarshalInfo()
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}
		if info.Private != nil {
			t.Errorf("Expected private field to be nil (removed), got %v", *info.Private)
		}
	})

	t.Run("ClearSource", func(t *testing.T) {
		outPath := filepath.Join(tmpDir, "cleared_source.torrent")
		result, err := ModifyTorrent(torrentPath, ModifyOptions{
			Source:        "",
			SourceSet:     true,
			OutputDir:     tmpDir,
			OutputPattern: "cleared_source",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified")
		}

		// verify source field is cleared
		mi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		info, err := mi.UnmarshalInfo()
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}
		if info.Source != "" {
			t.Errorf("Expected source to be empty, got %q", info.Source)
		}

		// verify source key is not present in raw bencode
		infoMap := make(map[string]interface{})
		if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
			t.Fatalf("Failed to unmarshal info bytes: %v", err)
		}
		if _, exists := infoMap["source"]; exists {
			t.Error("Expected source key to be absent from info dict")
		}
	})

	t.Run("ClearComment", func(t *testing.T) {
		outPath := filepath.Join(tmpDir, "cleared_comment.torrent")
		result, err := ModifyTorrent(torrentPath, ModifyOptions{
			Comment:       "",
			CommentSet:    true,
			OutputDir:     tmpDir,
			OutputPattern: "cleared_comment",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified")
		}

		// verify comment is cleared
		mi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		if mi.Comment != "" {
			t.Errorf("Expected comment to be empty, got %q", mi.Comment)
		}
	})

	t.Run("RemovePrivateAndClearSourceAndComment", func(t *testing.T) {
		outPath := filepath.Join(tmpDir, "removed_all.torrent")
		result, err := ModifyTorrent(torrentPath, ModifyOptions{
			RemovePrivate: true,
			Source:        "",
			SourceSet:     true,
			Comment:       "",
			CommentSet:    true,
			OutputDir:     tmpDir,
			OutputPattern: "removed_all",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified")
		}

		mi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		info, err := mi.UnmarshalInfo()
		if err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}

		if info.Private != nil {
			t.Errorf("Expected private to be nil, got %v", *info.Private)
		}
		if info.Source != "" {
			t.Errorf("Expected source to be empty, got %q", info.Source)
		}
		if mi.Comment != "" {
			t.Errorf("Expected comment to be empty, got %q", mi.Comment)
		}
	})

	t.Run("PreserveEntropyWhenRemovingPrivate", func(t *testing.T) {
		// create a torrent with entropy enabled
		entropyTorrentPath := filepath.Join(tmpDir, "entropy_test.torrent")
		_, err := Create(CreateOptions{
			Path:       tmpDir,
			OutputPath: entropyTorrentPath,
			IsPrivate:  true,
			Source:     "SRC",
			NoDate:     true,
			Entropy:    true,
		})
		if err != nil {
			t.Fatalf("Failed to create entropy torrent: %v", err)
		}

		// verify entropy exists in original
		origMi, err := LoadFromFile(entropyTorrentPath)
		if err != nil {
			t.Fatalf("Failed to load original torrent: %v", err)
		}
		origMap := make(map[string]any)
		if err := bencode.Unmarshal(origMi.InfoBytes, &origMap); err != nil {
			t.Fatalf("Failed to unmarshal original info: %v", err)
		}
		origEntropy, hasEntropy := origMap["entropy"]
		if !hasEntropy {
			t.Fatal("Expected entropy key in original torrent")
		}

		// remove private and clear source — entropy must survive
		outPath := filepath.Join(tmpDir, "entropy_preserved.torrent")
		result, err := ModifyTorrent(entropyTorrentPath, ModifyOptions{
			RemovePrivate: true,
			Source:        "",
			SourceSet:     true,
			OutputDir:     tmpDir,
			OutputPattern: "entropy_preserved",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified")
		}

		// verify entropy is preserved
		mi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		modMap := make(map[string]any)
		if err := bencode.Unmarshal(mi.InfoBytes, &modMap); err != nil {
			t.Fatalf("Failed to unmarshal modified info: %v", err)
		}
		modEntropy, hasEntropy := modMap["entropy"]
		if !hasEntropy {
			t.Error("Expected entropy key to be preserved after removing private/source")
		}
		if fmt.Sprintf("%v", origEntropy) != fmt.Sprintf("%v", modEntropy) {
			t.Errorf("Entropy value changed: %v -> %v", origEntropy, modEntropy)
		}

		// verify private and source are gone
		if _, exists := modMap["private"]; exists {
			t.Error("Expected private key to be removed")
		}
		if _, exists := modMap["source"]; exists {
			t.Error("Expected source key to be removed")
		}
	})

	t.Run("ClearAlreadyEmptySource", func(t *testing.T) {
		// create a torrent without source, then manually inject an empty source key
		noSrcTorrentPath := filepath.Join(tmpDir, "empty_source.torrent")
		_, err := Create(CreateOptions{
			Path:       tmpDir,
			OutputPath: noSrcTorrentPath,
			IsPrivate:  true,
			Source:     "PLACEHOLDER",
			NoDate:     true,
		})
		if err != nil {
			t.Fatalf("Failed to create torrent: %v", err)
		}

		// manually set source to empty string in the raw bencode (simulating a torrent
		// that has the key present but with empty value)
		mi, err := LoadFromFile(noSrcTorrentPath)
		if err != nil {
			t.Fatalf("Failed to load torrent: %v", err)
		}
		infoMap := make(map[string]any)
		if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}
		infoMap["source"] = "" // set empty source key
		infoBytes, err := bencode.Marshal(infoMap)
		if err != nil {
			t.Fatalf("Failed to marshal modified info: %v", err)
		}
		mi.InfoBytes = infoBytes

		// save the modified torrent
		emptySourcePath := filepath.Join(tmpDir, "has_empty_source.torrent")
		f, err := os.Create(emptySourcePath)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		if err := mi.Write(f); err != nil {
			f.Close()
			t.Fatalf("Failed to write torrent: %v", err)
		}
		f.Close()

		// verify the empty source key exists before modification
		preMi, _ := LoadFromFile(emptySourcePath)
		preMap := make(map[string]any)
		bencode.Unmarshal(preMi.InfoBytes, &preMap)
		if _, exists := preMap["source"]; !exists {
			t.Fatal("Setup failed: expected source key to exist (even if empty)")
		}

		// now use --source "" to clear it
		outPath := filepath.Join(tmpDir, "cleared_empty_source.torrent")
		result, err := ModifyTorrent(emptySourcePath, ModifyOptions{
			Source:        "",
			SourceSet:     true,
			OutputDir:     tmpDir,
			OutputPattern: "cleared_empty_source",
			Version:       "test",
		})
		if err != nil {
			t.Fatalf("ModifyTorrent failed: %v", err)
		}
		if !result.WasModified {
			t.Fatal("Expected torrent to be modified even when source was already empty")
		}

		// verify the source key is completely removed from the raw bencode
		postMi, err := LoadFromFile(outPath)
		if err != nil {
			t.Fatalf("Failed to load modified torrent: %v", err)
		}
		postMap := make(map[string]any)
		if err := bencode.Unmarshal(postMi.InfoBytes, &postMap); err != nil {
			t.Fatalf("Failed to unmarshal info: %v", err)
		}
		if _, exists := postMap["source"]; exists {
			t.Error("Expected source key to be completely removed from info dict")
		}
	})
}
