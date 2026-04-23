package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/preset"
)

func Test_calculatePieceLength(t *testing.T) {
	tests := []struct {
		name           string
		totalSize      int64
		maxPieceLength *uint
		trackerURLs    []string
		want           uint
		wantPieces     *uint // expected number of pieces (approximate)
	}{
		{
			name:      "small file should use minimum piece length",
			totalSize: 1 << 10, // 1 KiB
			want:      15,      // 32 KiB pieces
		},
		{
			name:      "63MB file should use 32KiB pieces",
			totalSize: 63 << 20,
			want:      15, // 32 KiB pieces
		},
		{
			name:      "65MB file should use 64KiB pieces",
			totalSize: 65 << 20,
			want:      16, // 64 KiB pieces
		},
		{
			name:      "129MB file should use 128KiB pieces",
			totalSize: 129 << 20,
			want:      17, // 128 KiB pieces
		},
		{
			name:      "257MB file should use 256KiB pieces",
			totalSize: 257 << 20,
			want:      18, // 256 KiB pieces
		},
		{
			name:      "513MB file should use 512KiB pieces",
			totalSize: 513 << 20,
			want:      19, // 512 KiB pieces
		},
		{
			name:      "1.1GB file should use 1MiB pieces",
			totalSize: 1100 << 20,
			want:      20, // 1 MiB pieces
		},
		{
			name:      "2.1GB file should use 2MiB pieces",
			totalSize: 2100 << 20,
			want:      21, // 2 MiB pieces
		},
		{
			name:      "4.1GB file should use 4MiB pieces",
			totalSize: 4100 << 20,
			want:      22, // 4 MiB pieces
		},
		{
			name:      "8.1GB file should use 8MiB pieces",
			totalSize: 8200 << 20,
			want:      23, // 8 MiB pieces
		},
		{
			name:      "16.1GB file should use 16MiB pieces",
			totalSize: 16500 << 20,
			want:      24, // 16 MiB pieces
		},
		{
			name:      "256.1GB file should use 16MiB pieces by default",
			totalSize: 256100 << 20, // 256.1 GB
			want:      24,           // 16 MiB pieces
		},
		{
			name:        "emp should respect max piece length of 2^23",
			totalSize:   100 << 30, // 100 GiB
			trackerURLs: []string{"https://empornium.sx/announce?passkey=123"},
			want:        23, // limited to 8 MiB pieces
		},
		{
			name:        "unknown tracker should use default calculation",
			totalSize:   10 << 30, // 10 GiB
			trackerURLs: []string{"https://unknown.tracker.com/announce"},
			want:        23, // 8 MiB pieces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePieceLength(tt.totalSize, tt.maxPieceLength, tt.trackerURLs, false)
			if got != tt.want {
				t.Errorf("calculatePieceLength() = %v, want %v", got, tt.want)
			}

			// verify the piece count is within reasonable bounds when targeting pieces
			if tt.wantPieces != nil {
				pieceLen := int64(1) << got
				pieces := (tt.totalSize + pieceLen - 1) / pieceLen

				// verify we're within 10% of expected piece count
				ratio := float64(pieces) / float64(*tt.wantPieces)
				if ratio < 0.9 || ratio > 1.1 {
					t.Errorf("pieces count too far from expected: got %v pieces, expected %v (ratio %.2f)",
						pieces, *tt.wantPieces, ratio)
				}
			}
		})
	}
}

func Test_calculatePieceLengthFromTarget(t *testing.T) {
	tests := []struct {
		name           string
		totalSize      int64
		targetCount    uint
		maxPieceLength *uint
		trackerURLs    []string
		wantExp        uint
	}{
		{
			name:        "10GB with target 1000 pieces",
			totalSize:   10 << 30,
			targetCount: 1000,
			wantExp:     23, // 8 MiB → ~1280 pieces
		},
		{
			name:        "1MiB with target 1000 pieces clamps to minimum",
			totalSize:   1 << 20,
			targetCount: 1000,
			wantExp:     16, // 64 KiB minimum
		},
		{
			name:        "100GB with target 50 clamps to default max",
			totalSize:   100 << 30,
			targetCount: 50,
			wantExp:     24, // 16 MiB default cap
		},
		{
			name:        "totalSize smaller than targetCount",
			totalSize:   100,
			targetCount: 1000,
			wantExp:     16, // ratio=0, clamps to min
		},
		{
			name:        "exact power of 2: 8GB target 1024",
			totalSize:   8 << 30,
			targetCount: 1024,
			wantExp:     23, // 8 MiB exactly
		},
		{
			name:        "tracker cap: emp limits to 2^23",
			totalSize:   100 << 30,
			targetCount: 50,
			trackerURLs: []string{"https://empornium.sx/announce?passkey=123"},
			wantExp:     23, // emp max is 23
		},
		{
			name:           "maxPieceLength lowers ceiling",
			totalSize:      100 << 30,
			targetCount:    50,
			maxPieceLength: uintPtr(22),
			wantExp:        22,
		},
		{
			name:           "maxPieceLength raises ceiling above default 24 (no tracker)",
			totalSize:      100 << 30,
			targetCount:    50,
			maxPieceLength: uintPtr(27),
			wantExp:        27, // 100GB/50 ≈ 2GB = 2^31, floor(log2) = 30, clamped to 27
		},
		{
			name:           "maxPieceLength cannot exceed tracker cap",
			totalSize:      100 << 30,
			targetCount:    50,
			maxPieceLength: uintPtr(27),
			trackerURLs:    []string{"https://empornium.sx/announce?passkey=123"},
			wantExp:        23, // emp cap is 23, user's 27 is clamped down
		},
		{
			name:        "target 1 piece gives max exp",
			totalSize:   10 << 30,
			targetCount: 1,
			wantExp:     24, // capped at default max (no maxPieceLength set)
		},
		{
			name:        "4GB with target 500",
			totalSize:   4 << 30,
			targetCount: 500,
			wantExp:     23, // ~4GB/500 = ~8MiB = 2^23
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePieceLengthFromTarget(tt.totalSize, tt.targetCount, tt.maxPieceLength, tt.trackerURLs, false)
			if got != tt.wantExp {
				t.Errorf("calculatePieceLengthFromTarget() = %v, want %v", got, tt.wantExp)
			}
		})
	}
}

func TestCreateTorrent_TargetPieceCount(t *testing.T) {
	// integration test: create a real torrent with target piece count
	dir := t.TempDir()

	// create a 4 MiB file
	data := make([]byte, 4<<20)
	if err := os.WriteFile(filepath.Join(dir, "testfile.bin"), data, 0644); err != nil {
		t.Fatal(err)
	}

	target := uint(100)
	tor, err := CreateTorrent(CreateOptions{
		Path:             dir,
		TargetPieceCount: &target,
		IsPrivate:        true,
		NoDate:           true,
		NoCreator:        true,
		Version:          "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	info, err := tor.UnmarshalInfo()
	if err != nil {
		t.Fatal(err)
	}

	numPieces := len(info.Pieces) / 20 // SHA1 hash is 20 bytes
	// with 4 MiB content and target 100 pieces, we expect exp=16 (64 KiB)
	// which gives 4 MiB / 64 KiB = 64 pieces
	// (ratio = 4MiB/100 ≈ 40KiB, floor(log2(40K)) = 15, clamped to min 16)
	if numPieces < 1 || numPieces > 200 {
		t.Errorf("unexpected piece count %d for 4 MiB content with target 100", numPieces)
	}

	if info.PieceLength == 0 {
		t.Error("piece length should not be zero")
	}
}

func TestCreateTorrent_TargetPieceCountZero(t *testing.T) {
	dir := t.TempDir()
	data := make([]byte, 1<<20)
	if err := os.WriteFile(filepath.Join(dir, "testfile.bin"), data, 0644); err != nil {
		t.Fatal(err)
	}

	zero := uint(0)
	_, err := CreateTorrent(CreateOptions{
		Path:             dir,
		TargetPieceCount: &zero,
		IsPrivate:        true,
		NoDate:           true,
		NoCreator:        true,
		Version:          "test",
	})
	if err == nil {
		t.Fatal("expected error for target piece count of zero")
	}
	if !strings.Contains(err.Error(), "greater than zero") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func uintPtr(v uint) *uint { return &v }

func TestCreateTorrent_Symlink(t *testing.T) {
	// Skip symlink tests on Windows as it requires special privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	// 1. Setup temporary directory structure
	tmpDir, err := os.MkdirTemp("", "mkbrr-symlink-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create real content directory and file
	realContentDir := filepath.Join(tmpDir, "real_content")
	if err := os.Mkdir(realContentDir, 0755); err != nil {
		t.Fatalf("Failed to create real_content dir: %v", err)
	}
	realFilePath := filepath.Join(realContentDir, "file.txt")
	fileContent := []byte("This is the actual content of the file.")
	if err := os.WriteFile(realFilePath, fileContent, 0644); err != nil {
		t.Fatalf("Failed to write real file: %v", err)
	}
	realFileInfo, _ := os.Stat(realFilePath) // Get real file size

	// Create directory to contain the symlink
	linkDir := filepath.Join(tmpDir, "link_dir")
	if err := os.Mkdir(linkDir, 0755); err != nil {
		t.Fatalf("Failed to create link_dir: %v", err)
	}

	// Create the symlink pointing to the real file (relative path)
	linkPath := filepath.Join(linkDir, "link_to_file.txt")
	linkTarget := "../real_content/file.txt"
	if err := os.Symlink(linkTarget, linkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// 2. Create Torrent Options
	pieceLenExp := uint(16) // 64 KiB pieces
	opts := CreateOptions{
		Path:           linkDir, // Create torrent from the directory containing the link
		OutputPath:     filepath.Join(tmpDir, "symlink_test.torrent"),
		IsPrivate:      true,
		NoCreator:      true,
		NoDate:         true,
		PieceLengthExp: &pieceLenExp,
	}

	// 3. Create the torrent
	createdTorrentInfo, err := Create(opts)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// 4. Verification
	// Load the created torrent file
	mi, err := metainfo.LoadFromFile(createdTorrentInfo.Path)
	if err != nil {
		t.Fatalf("Failed to load created torrent file %q: %v", createdTorrentInfo.Path, err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		t.Fatalf("Failed to unmarshal info from created torrent: %v", err)
	}

	// Verify torrent structure (should contain the link name, not the target name)
	if len(info.Files) != 1 {
		t.Fatalf("Expected 1 file in torrent info, got %d", len(info.Files))
	}
	expectedPathInTorrent := []string{"link_to_file.txt"}
	if !reflect.DeepEqual(info.Files[0].Path, expectedPathInTorrent) {
		t.Errorf("Expected file path in torrent %v, got %v", expectedPathInTorrent, info.Files[0].Path)
	}

	// Verify file length matches the *target* file's length
	if info.Files[0].Length != realFileInfo.Size() {
		t.Errorf("Expected file length %d (target size), got %d", realFileInfo.Size(), info.Files[0].Length)
	}

	// Verify piece hash matches the *target* file's content
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (realFileInfo.Size() + pieceLen - 1) / pieceLen
	if int(numPieces) != len(info.Pieces)/20 {
		t.Fatalf("Piece count mismatch: expected %d, got %d pieces in torrent", numPieces, len(info.Pieces)/20)
	}

	// Since the content is smaller than the piece size, the hash is just the hash of the content
	// padded with zeros to the piece length if necessary (though CreateTorrent handles this internally).
	// For simplicity here, we hash just the content as it fits in one piece.
	hasher := sha1.New()
	hasher.Write(fileContent)
	expectedHash := hasher.Sum(nil)

	// The actual piece hash in the torrent might be padded if piece length > content length.
	// We need to compare against the actual hash stored.
	actualPieceHash := info.Pieces[:20] // Get the first (and only) piece hash

	if !bytes.Equal(actualPieceHash, expectedHash) {
		t.Errorf("Piece hash mismatch:\nExpected: %x\nGot:      %x", expectedHash, actualPieceHash)
	}

	t.Logf("Symlink test successful: Torrent created from %q, correctly referencing content from %q", linkDir, realFilePath)
}

func TestCreateTorrent_IgnoresSynologyMetadataDir(t *testing.T) {
	// Setup temporary directory with a regular file and Synology metadata directory
	rootDir := t.TempDir()

	regularFile := filepath.Join(rootDir, "movie.mkv")
	if err := os.WriteFile(regularFile, []byte("video data"), 0o644); err != nil {
		t.Fatalf("failed to write regular file: %v", err)
	}

	metaDir := filepath.Join(rootDir, "@eaDir")
	if err := os.Mkdir(metaDir, 0o755); err != nil {
		t.Fatalf("failed to create @eaDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "metadata.txt"), []byte("should be ignored"), 0o644); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	// create nested structure to ensure recursive ignores
	nestedDir := filepath.Join(metaDir, "subdir")
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "deep.txt"), []byte("ignore me too"), 0o644); err != nil {
		t.Fatalf("failed to write nested metadata file: %v", err)
	}

	opts := CreateOptions{
		Path:      rootDir,
		NoCreator: true,
		NoDate:    true,
	}

	tor, err := CreateTorrent(opts)
	if err != nil {
		t.Fatalf("CreateTorrent failed: %v", err)
	}

	info := tor.GetInfo()
	if len(info.Files) != 1 {
		var paths []string
		for _, f := range info.Files {
			paths = append(paths, strings.Join(f.Path, "/"))
		}
		t.Fatalf("expected 1 file, got %d: %v", len(info.Files), paths)
	}

	gotPath := strings.Join(info.Files[0].Path, "/")
	if gotPath != "movie.mkv" {
		t.Fatalf("expected movie.mkv in torrent, got %q", gotPath)
	}
	if strings.Contains(strings.ToLower(gotPath), "@eadir") {
		t.Fatalf("@eaDir directory unexpectedly included in torrent path %q", gotPath)
	}
}

func TestCreateTorrent_SingleFilePatterns(t *testing.T) {
	rootDir := t.TempDir()
	filePath := filepath.Join(rootDir, "movie.mkv")
	if err := os.WriteFile(filePath, []byte("video data"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	tests := []struct {
		name            string
		excludePatterns []string
		includePatterns []string
		wantErr         bool
	}{
		{
			name:            "exclude matching single file",
			excludePatterns: []string{"*.mkv"},
			wantErr:         true,
		},
		{
			name:            "include non-matching single file",
			includePatterns: []string{"*.mp4"},
			wantErr:         true,
		},
		{
			name:            "include matching single file",
			includePatterns: []string{"*.mkv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := CreateOptions{
				Path:            filePath,
				ExcludePatterns: tt.excludePatterns,
				IncludePatterns: tt.includePatterns,
				NoCreator:       true,
				NoDate:          true,
			}

			tor, err := CreateTorrent(opts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateTorrent() error = %v", err)
			}
			if tor.GetInfo().Length == 0 {
				t.Fatalf("expected single-file torrent length to be set")
			}
		})
	}
}

func TestCreateTorrent_OutputDirPriority(t *testing.T) {
	// Setup temporary directories for test
	tmpDir, err := os.MkdirTemp("", "mkbrr-create-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a non-empty file in the temp directory for the torrent content
	dummyFilePath := filepath.Join(tmpDir, "dummy.txt")
	if err := os.WriteFile(dummyFilePath, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy file: %v", err)
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
		opts           CreateOptions
		expectedOutDir string
	}{
		{
			name: "Command-line OutputDir should take precedence",
			opts: CreateOptions{
				Path:      tmpDir,
				OutputDir: cmdLineOutputDir,
				IsPrivate: true,
				NoDate:    true,
				NoCreator: true,
			},
			expectedOutDir: cmdLineOutputDir,
		},
		{
			name: "Preset OutputDir should be used when no command-line OutputDir",
			opts: CreateOptions{
				Path:      tmpDir,
				OutputDir: "", // empty to simulate preset usage
				IsPrivate: true,
				NoDate:    true,
				NoCreator: true,
			},
			expectedOutDir: presetOutputDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the preset test case, we need to simulate the preset loading
			if tt.name == "Preset OutputDir should be used when no command-line OutputDir" {
				// Load preset options and apply them
				presetOpts, err := preset.LoadPresetOptions(presetPath, "test")
				if err != nil {
					t.Fatalf("Failed to load preset options: %v", err)
				}

				// Apply preset OutputDir if command-line OutputDir is empty
				if tt.opts.OutputDir == "" && presetOpts.OutputDir != "" {
					tt.opts.OutputDir = presetOpts.OutputDir
				}
			}

			result, err := Create(tt.opts)
			if err != nil {
				t.Fatalf("Create failed: %v", err)
			}

			// Verify the output path contains the expected directory
			dir := filepath.Dir(result.Path)
			if dir != tt.expectedOutDir {
				t.Errorf("Expected output directory %q, got %q", tt.expectedOutDir, dir)
			}

			// Verify the file was actually created in the expected directory
			if _, err := os.Stat(result.Path); os.IsNotExist(err) {
				t.Errorf("Output file wasn't created at expected path: %s", result.Path)
			}
		})
	}
}

func TestCreate_MultipleTrackers(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "mkbrr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content for multiple trackers"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name              string
		trackers          []string
		expectedAnnounce  string
		expectedTierCount int
	}{
		{
			name:              "Single tracker",
			trackers:          []string{"https://tracker1.com/announce"},
			expectedAnnounce:  "https://tracker1.com/announce",
			expectedTierCount: 0,
		},
		{
			name:              "Multiple trackers",
			trackers:          []string{"https://tracker1.com/announce", "https://tracker2.com/announce", "https://tracker3.com/announce"},
			expectedAnnounce:  "https://tracker1.com/announce",
			expectedTierCount: 3,
		},
		{
			name:              "No trackers",
			trackers:          []string{},
			expectedAnnounce:  "",
			expectedTierCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pieceLenExp := uint(16)
			outputPath := filepath.Join(tmpDir, tt.name+".torrent")

			opts := CreateOptions{
				Path:           tmpDir,
				TrackerURLs:    tt.trackers,
				OutputPath:     outputPath,
				IsPrivate:      true,
				NoCreator:      true,
				NoDate:         true,
				PieceLengthExp: &pieceLenExp,
			}

			// Create the torrent
			result, err := Create(opts)
			if err != nil {
				t.Fatalf("Create() failed: %v", err)
			}

			// Load the created torrent file
			mi, err := metainfo.LoadFromFile(result.Path)
			if err != nil {
				t.Fatalf("Failed to load created torrent file: %v", err)
			}

			// Check announce URL
			if mi.Announce != tt.expectedAnnounce {
				t.Errorf("Expected announce URL %q, got %q", tt.expectedAnnounce, mi.Announce)
			}

			// Check announce list
			if len(tt.trackers) > 1 {
				if mi.AnnounceList == nil || len(mi.AnnounceList) != tt.expectedTierCount {
					t.Errorf("Expected AnnounceList with %d tiers, got %v", tt.expectedTierCount, mi.AnnounceList)
				} else {
					for i, tracker := range tt.trackers {
						if len(mi.AnnounceList[i]) != 1 {
							t.Errorf("Expected tier %d to contain 1 tracker, got %v", i, mi.AnnounceList[i])
							continue
						}
						if mi.AnnounceList[i][0] != tracker {
							t.Errorf("Expected tracker %q at tier %d, got %q", tracker, i, mi.AnnounceList[i][0])
						}
					}
				}
			} else {
				// No announce list should be set when there are zero or one trackers
				if len(mi.AnnounceList) > 0 {
					t.Errorf("Expected empty announce list, got %v", mi.AnnounceList)
				}
			}

			// Check result announce field
			if result.Announce != tt.expectedAnnounce {
				t.Errorf("Expected result announce %q, got %q", tt.expectedAnnounce, result.Announce)
			}
		})
	}
}

func TestCreate_UsesCustomNameForOutputPath(t *testing.T) {
	t.Parallel()

	const customName = "CustomShow"
	content := []byte("tiny sample so the test stays fast")

	cases := []struct {
		scenario string
		trackers []string
		wantFile string
	}{
		{
			scenario: "when I pick a custom name without any tracker, the torrent file should use it",
			trackers: nil,
			wantFile: customName + ".torrent",
		},
		{
			scenario: "when I pick a custom name and add a tracker, the tracker prefix should still keep my name",
			trackers: []string{"https://tracker.example.com/announce"},
			wantFile: "example_" + customName + ".torrent",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()

			workspace := t.TempDir()
			inputPath := filepath.Join(workspace, "video.mkv")
			if err := os.WriteFile(inputPath, content, 0644); err != nil {
				t.Fatalf("failed to write input file: %v", err)
			}

			outputDir := filepath.Join(workspace, "out")
			opts := CreateOptions{
				Path:        inputPath,
				Name:        customName,
				TrackerURLs: tc.trackers,
				OutputDir:   outputDir,
				Quiet:       true,
			}

			info, err := Create(opts)
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}

			gotFile := filepath.Base(info.Path)
			if gotFile != tc.wantFile {
				t.Fatalf("expected torrent output %q, got %q", tc.wantFile, gotFile)
			}

			if _, err := os.Stat(info.Path); err != nil {
				t.Fatalf("expected torrent file to exist at %q, got error: %v", info.Path, err)
			}
		})
	}
}

func TestCreate_NameArgument(t *testing.T) {

	tracker := "https://unknown.customtracker.com/announce"
	tracker2 := "https://unknown.customtracker2.com/announce"

	// Create temporary directory
	testDir, err := os.MkdirTemp("", "mkbrr-create-TestCreate-NameArgument-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	baseDir := filepath.Base(testDir)
	defer os.RemoveAll(testDir)

	// Create test files
	filename := "test.txt"
	testFile := filepath.Join(testDir, filename)
	if err := os.WriteFile(testFile, []byte("create test with -name argument"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	filename2 := "test2.txt"
	testFile2 := filepath.Join(testDir, filename2)
	if err := os.WriteFile(testFile2, []byte("create test with -name argument #2"), 0644); err != nil {
		t.Fatalf("Failed to create test file2: %v", err)
	}

	// Test cases
	tests := []struct {
		name             string
		opts             CreateOptions
		expectedName     string
		expectedFilename string
	}{
		{
			name: "Single file with no --name argument --skip-prefix not present no -o",
			opts: CreateOptions{
				Path:        testFile,
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     filename,
			expectedFilename: "customtracker_" + filename + ".torrent",
		},
		{
			name: "Single file with no --name argument --skip-prefix present -o supplied",
			opts: CreateOptions{
				Path:        testFile,
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     filename,
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Single file with --name argument --skip-prefix not present no -o",
			opts: CreateOptions{
				Path:        testFile,
				Name:        "customname",
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customtracker_customname.torrent",
		},
		{
			name: "Single file with --name argument --skip-prefix present -o supplied, (-o overrides --skip-prefix)",
			opts: CreateOptions{
				Path:        testFile,
				Name:        "customname",
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Single file with --name argument --skip-prefix not present -o supplied",
			opts: CreateOptions{
				Path:        testFile,
				Name:        "customname",
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Single file multiple trackers no --name argument --skip-prefix not present -o supplied",
			opts: CreateOptions{
				Path:        testFile,
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker, tracker2},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     filename,
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Single file multiple trackers no --name argument --skip-prefix present no -o",
			opts: CreateOptions{
				Path:        testFile,
				TrackerURLs: []string{tracker, tracker2},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     filename,
			expectedFilename: filename + ".torrent",
		},
		{
			name: "Single file multiple trackers with --name argument --skip-prefix not present -o supplied",
			opts: CreateOptions{
				Path:        testFile,
				Name:        "customname",
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker, tracker2},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Single file multiple trackers with --name argument --skip-prefix present no -o",
			opts: CreateOptions{
				Path:        testFile,
				Name:        "customname",
				TrackerURLs: []string{tracker, tracker2},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customname.torrent",
		},
		{
			name: "Multiple files no --name argument no --skip-prefix no -o",
			opts: CreateOptions{
				Path:        testDir,
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     baseDir,
			expectedFilename: "customtracker_" + baseDir + ".torrent",
		},
		{
			name: "Multiple files no --name argument --skip-prefix present -o supplied",
			opts: CreateOptions{
				Path:        testDir,
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     baseDir,
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Multiple files with --name argument no --skip-prefix no -o",
			opts: CreateOptions{
				Path:        testDir,
				Name:        "customname",
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customtracker_customname.torrent",
		},
		{
			name: "Multiple files with --name argument --skip-prefix present no -o",
			opts: CreateOptions{
				Path:        testDir,
				Name:        "customname",
				TrackerURLs: []string{tracker},
				SkipPrefix:  true,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customname.torrent",
		},
		{
			name: "Multiple files with --name argument no --skip-prefix -o supplied",
			opts: CreateOptions{
				Path:        testDir,
				Name:        "customname",
				OutputPath:  filepath.Join(testDir, "customfilename"),
				TrackerURLs: []string{tracker},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customfilename.torrent",
		},
		{
			name: "Multiple files multiple trackers with --name argument no --skip-prefix no -o",
			opts: CreateOptions{
				Path:        testDir,
				Name:        "customname",
				TrackerURLs: []string{tracker, tracker2},
				SkipPrefix:  false,
				Quiet:       true,
			},
			expectedName:     "customname",
			expectedFilename: "customname.torrent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create the torrent
			result, err := Create(tt.opts)
			if err != nil {
				t.Fatalf("Create() failed: %v", err)
			}

			// Verify the file was actually created
			if _, err := os.Stat(result.Path); err != nil {
				t.Fatalf("Created torrent file does not exist: %v", err)
			}

			// Load the created torrent file
			mi, err := metainfo.LoadFromFile(result.Path)
			if err != nil {
				t.Fatalf("Failed to load created torrent file: %v", err)
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
			createdFilename := filepath.Base(result.Path)
			if createdFilename != tt.expectedFilename {
				t.Fatalf("Expected output filename %q, got %q", tt.expectedFilename, createdFilename)
			}

			t.Logf("Torrent created with name %q and filename %q as expected.", info.Name, createdFilename)
			os.Remove(result.Path) // best-effort cleanup; defer handles the rest
		})
	}
}
