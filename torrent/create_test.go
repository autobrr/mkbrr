package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
		expectedListCount int
	}{
		{
			name:              "Single tracker",
			trackers:          []string{"https://tracker1.com/announce"},
			expectedAnnounce:  "https://tracker1.com/announce",
			expectedListCount: 1,
		},
		{
			name:              "Multiple trackers",
			trackers:          []string{"https://tracker1.com/announce", "https://tracker2.com/announce", "https://tracker3.com/announce"},
			expectedAnnounce:  "https://tracker1.com/announce",
			expectedListCount: 3,
		},
		{
			name:              "No trackers",
			trackers:          []string{},
			expectedAnnounce:  "",
			expectedListCount: 0,
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
			if len(tt.trackers) > 0 {
				if mi.AnnounceList == nil || len(mi.AnnounceList) != 1 {
					t.Errorf("Expected AnnounceList with 1 tier, got %v", mi.AnnounceList)
				} else if len(mi.AnnounceList[0]) != tt.expectedListCount {
					t.Errorf("Expected %d trackers in announce list, got %d", tt.expectedListCount, len(mi.AnnounceList[0]))
				} else {
					// Check all trackers are in the announce list
					for i, tracker := range tt.trackers {
						if mi.AnnounceList[0][i] != tracker {
							t.Errorf("Expected tracker %q at position %d, got %q", tracker, i, mi.AnnounceList[0][i])
						}
					}
				}
			} else {
				// No trackers case
				if len(mi.AnnounceList) > 0 && len(mi.AnnounceList[0]) > 0 {
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
