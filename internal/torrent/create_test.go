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
)

func Test_calculatePieceLength(t *testing.T) {
	tests := []struct {
		name           string
		totalSize      int64
		maxPieceLength *uint
		trackerURL     string
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
			name:       "emp should respect max piece length of 2^23",
			totalSize:  100 << 30, // 100 GiB
			trackerURL: "https://empornium.sx/announce?passkey=123",
			want:       23, // limited to 8 MiB pieces
		},
		{
			name:       "unknown tracker should use default calculation",
			totalSize:  10 << 30, // 10 GiB
			trackerURL: "https://unknown.tracker.com/announce",
			want:       23, // 8 MiB pieces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePieceLength(tt.totalSize, tt.maxPieceLength, tt.trackerURL, false)
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
	opts := CreateTorrentOptions{
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
