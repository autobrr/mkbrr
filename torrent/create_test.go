// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/trackers"
)

func TestPieceCountForSizeHandlesIntegerBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		totalSize   int64
		pieceLength int64
		want        int
		wantErr     bool
	}{
		{name: "maximum piece length", totalSize: 2, pieceLength: maxTorrentDataSize, want: 1},
		{name: "excessive piece count", totalSize: maxTorrentDataSize, pieceLength: 1, wantErr: true},
		{name: "negative total size", totalSize: -1, pieceLength: 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := pieceCountForSize(test.totalSize, test.pieceLength)
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestAddTorrentFileSizeRejectsOverflow(t *testing.T) {
	tests := []struct {
		name      string
		totalSize int64
		fileSize  int64
		want      int64
		wantErr   bool
	}{
		{name: "maximum total size", totalSize: maxTorrentDataSize - 1, fileSize: 1, want: maxTorrentDataSize},
		{name: "overflow", totalSize: maxTorrentDataSize, fileSize: 1, wantErr: true},
		{name: "negative file size", totalSize: 0, fileSize: -1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := addTorrentFileSize(test.totalSize, test.fileSize)
			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func Test_choosePieceLength_Automatic(t *testing.T) {
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
			name:        "ggn custom range should allow 32KiB pieces",
			totalSize:   32 << 20,
			trackerURLs: []string{"https://gazellegames.net/announce?passkey=123"},
			want:        15,
		},
		{
			name:        "bhd default ranges keep historical 64KiB tracker minimum",
			totalSize:   32 << 20,
			trackerURLs: []string{"https://beyond-hd.me/announce?passkey=123"},
			want:        16,
		},
		{
			name:           "user max remains authoritative for custom tracker range",
			totalSize:      100 << 30,
			maxPieceLength: new(uint(24)),
			trackerURLs:    []string{"https://gazellegames.net/announce?passkey=123"},
			want:           24,
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
			got, _, err := choosePieceLength(tt.totalSize, CreateOptions{MaxPieceLength: tt.maxPieceLength, TrackerURLs: tt.trackerURLs}, rulesFor(tt.trackerURLs), nil)
			require.NoError(t, err)
			if got != tt.want {
				t.Errorf("choosePieceLength() = %v, want %v", got, tt.want)
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

func Test_choosePieceLength_Target(t *testing.T) {
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
			maxPieceLength: new(uint(22)),
			wantExp:        22,
		},
		{
			name:           "maxPieceLength raises ceiling above default 24 (no tracker)",
			totalSize:      100 << 30,
			targetCount:    50,
			maxPieceLength: new(uint(27)),
			wantExp:        27, // 100GB/50 ≈ 2GB = 2^31, floor(log2) = 30, clamped to 27
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
			opts := CreateOptions{TargetPieceCount: &tt.targetCount, MaxPieceLength: tt.maxPieceLength, TrackerURLs: tt.trackerURLs}
			got, _, err := choosePieceLength(tt.totalSize, opts, rulesFor(tt.trackerURLs), nil)
			require.NoError(t, err)
			if got != tt.wantExp {
				t.Errorf("choosePieceLength() = %v, want %v", got, tt.wantExp)
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

func TestGetRecommendedPieceLengthExpMatchesCreateMinimum(t *testing.T) {
	got := GetRecommendedPieceLengthExp("https://beyond-hd.me/announce?passkey=123", 32<<20)
	if got != 16 {
		t.Fatalf("GetRecommendedPieceLengthExp() = %d, want 16", got)
	}
}

func TestGetRecommendedPieceLengthExpCustomRanges(t *testing.T) {
	tests := []struct {
		name        string
		trackerURL  string
		contentSize uint64
		want        uint
	}{
		{"ggn small content uses 32 KiB", "https://gazellegames.net/announce?passkey=123", 32 << 20, 15},
		{"ggn large content keeps hard cap", "https://gazellegames.net/announce?passkey=123", 100 << 30, 26},
		{"portugas small content uses 16 KiB", "https://portugas.org/announce/passkey", 30 << 20, 14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetRecommendedPieceLengthExp(tt.trackerURL, tt.contentSize))
		})
	}
}

func TestGetRecommendedPieceLengthExpUnknownTracker(t *testing.T) {
	got := GetRecommendedPieceLengthExp("https://unknown.tracker/announce", 32<<20)
	if got != 0 {
		t.Fatalf("GetRecommendedPieceLengthExp() = %d, want 0", got)
	}
}

func rulesFor(trackerURLs []string) trackers.Rules {
	if len(trackerURLs) == 0 {
		return trackers.Rules{}
	}
	rules, _ := trackers.Lookup(trackerURLs[0])
	return rules
}

func Test_choosePieceLength_ExplicitBounds(t *testing.T) {
	// the trackers here recommend less than 64 KiB for 1 MiB of content;
	// the recommendation never rejects an explicit piece length
	tests := []struct {
		name       string
		trackerURL string
		exp        uint
		wantErr    bool
	}{
		{"ggn custom table accepts 64 KiB", "https://gazellegames.net/announce?passkey=123", 16, false},
		{"portugas custom table accepts 64 KiB", "https://portugas.org/announce/passkey", 16, false},
		{"bhd default ranges accept 64 KiB", "https://beyond-hd.me/announce?passkey=123", 16, false},
		{"ggn custom table accepts 32 KiB", "https://gazellegames.net/announce?passkey=123", 15, false},
		{"portugas custom table accepts 16 KiB", "https://portugas.org/announce/passkey", 14, false},
		{"bhd default ranges reject 32 KiB", "https://beyond-hd.me/announce?passkey=123", 15, true},
		{"portugas custom table rejects 8 KiB", "https://portugas.org/announce/passkey", 13, true},
		{"ggn hard cap rejects 128 MiB", "https://gazellegames.net/announce?passkey=123", 27, true},
		{"no tracker accepts 32 KiB", "", 15, false},
		{"no tracker rejects 16 KiB", "", 14, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var trackerURLs []string
			if tt.trackerURL != "" {
				trackerURLs = []string{tt.trackerURL}
			}
			got, _, err := choosePieceLength(1<<20, CreateOptions{TrackerURLs: trackerURLs, PieceLengthExp: new(tt.exp)}, rulesFor(trackerURLs), nil)
			if tt.wantErr {
				assert.ErrorContains(t, err, "piece length exponent must be between")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.exp, got)
		})
	}
}

func Test_choosePieceLength_SizeLimit(t *testing.T) {
	// a fake .torrent size: 1 KiB of other fields plus 20 bytes for each piece
	const totalSize = 10 << 30
	fakeSize := func(exp uint) (uint64, error) {
		return 1<<10 + 20*((totalSize+(1<<exp)-1)>>exp), nil
	}
	rules := trackers.Rules{MaxTorrentSize: 100 << 10}

	tests := []struct {
		name       string
		opts       CreateOptions
		rules      trackers.Rules
		want       uint
		wantRaised bool
		wantErr    bool
	}{
		{"fits without a change", CreateOptions{}, rules, 23, false, false},
		{"raises an explicit piece length", CreateOptions{PieceLengthExp: new(uint(16))}, rules, 22, true, false},
		{"fails when the ceiling is too low", CreateOptions{MaxPieceLength: new(uint(20))}, rules, 0, false, true},
		{"ignores the size without a limit", CreateOptions{PieceLengthExp: new(uint(16))}, trackers.Rules{}, 16, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notices, err := choosePieceLength(totalSize, tt.opts, tt.rules, fakeSize)
			if tt.wantErr {
				assert.ErrorContains(t, err, "unable to create torrent under size limit")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			raised := slices.ContainsFunc(notices, func(n Notice) bool { return n.Warn && strings.HasPrefix(n.Text, "raised piece length") })
			assert.Equal(t, tt.wantRaised, raised)
		})
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
		paths := make([]string, 0, len(info.Files))
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

func TestCreateTorrent_IgnoresActiveTempFilesOnly(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "movie.mkv"), []byte("video data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".mkbrr-notes.tmp"), []byte("user temp data"), 0o644))

	torrentFilePaths := func(tor *Torrent) []string {
		files := tor.GetInfo().Files
		paths := make([]string, 0, len(files))
		for _, f := range files {
			paths = append(paths, strings.Join(f.Path, "/"))
		}
		return paths
	}

	cleanTor, err := CreateTorrent(CreateOptions{
		Path:      rootDir,
		NoCreator: true,
		NoDate:    true,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{".mkbrr-notes.tmp", "movie.mkv"}, torrentFilePaths(cleanTor))

	const (
		activeRoot   = ".mkbrr-ABC123.tmp"
		activeNested = ".mkbrr-DEF456.tmp"
	)
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, activeRoot), []byte("probe"), 0o644))
	nestedDir := filepath.Join(rootDir, "nested")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, activeNested), []byte("probe"), 0o644))

	unregisterRoot := registerActiveTempTorrent(activeRoot)
	t.Cleanup(unregisterRoot)
	unregisterNested := registerActiveTempTorrent(activeNested)
	t.Cleanup(unregisterNested)

	t.Run("default filters keep user .mkbrr-notes.tmp and ignore active temp files", func(t *testing.T) {
		tor, err := CreateTorrent(CreateOptions{
			Path:      rootDir,
			NoCreator: true,
			NoDate:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{".mkbrr-notes.tmp", "movie.mkv"}, torrentFilePaths(tor))
		assert.Equal(t, cleanTor.HashInfoBytes(), tor.HashInfoBytes())
	})

	t.Run("include wildcard still ignores active temp files", func(t *testing.T) {
		tor, err := CreateTorrent(CreateOptions{
			Path:            rootDir,
			IncludePatterns: []string{"*"},
			NoCreator:       true,
			NoDate:          true,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{".mkbrr-notes.tmp", "movie.mkv"}, torrentFilePaths(tor))
	})

	t.Run("include *.tmp keeps user .mkbrr-notes.tmp and ignores active temp files", func(t *testing.T) {
		tor, err := CreateTorrent(CreateOptions{
			Path:            rootDir,
			IncludePatterns: []string{"*.tmp"},
			NoCreator:       true,
			NoDate:          true,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{".mkbrr-notes.tmp"}, torrentFilePaths(tor))
	})

	t.Run("cleanup stops suppressing the filename", func(t *testing.T) {
		unregisterRoot()
		tor, err := CreateTorrent(CreateOptions{
			Path:            rootDir,
			IncludePatterns: []string{"*.tmp"},
			NoCreator:       true,
			NoDate:          true,
		})
		require.NoError(t, err)
		assert.Equal(t, []string{activeRoot, ".mkbrr-notes.tmp"}, torrentFilePaths(tor))
	})
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

func TestCreateOutputPath(t *testing.T) {
	const tracker = "https://tracker.example.org/announce"
	const tracker2 = "https://tracker2.example.net/announce"
	content := filepath.Join("data", "Movie")
	absOut := filepath.Join(t.TempDir(), "abs.torrent")
	absOutNoExt := filepath.Join(t.TempDir(), "abs")

	tests := []struct {
		name   string
		opts   CreateOptions
		preset *preset.Options
		want   string
	}{
		{
			name: "default filename is the base name of the content path",
			opts: CreateOptions{Path: content},
			want: "Movie.torrent",
		},
		{
			name: "default filename of a single file keeps its extension",
			opts: CreateOptions{Path: filepath.Join("data", "Movie.mkv")},
			want: "Movie.mkv.torrent",
		},
		{
			name: "default filename ignores a trailing separator",
			opts: CreateOptions{Path: content + string(filepath.Separator)},
			want: "Movie.torrent",
		},
		{
			name: "name sets the default filename",
			opts: CreateOptions{Path: content, Name: "Custom"},
			want: "Custom.torrent",
		},
		{
			name: "one tracker adds the filename prefix",
			opts: CreateOptions{Path: content, TrackerURLs: []string{tracker}},
			want: "example_Movie.torrent",
		},
		{
			name: "one tracker adds the filename prefix before the name",
			opts: CreateOptions{Path: content, Name: "Custom", TrackerURLs: []string{tracker}},
			want: "example_Custom.torrent",
		},
		{
			name: "two trackers add no filename prefix",
			opts: CreateOptions{Path: content, TrackerURLs: []string{tracker, tracker2}},
			want: "Movie.torrent",
		},
		{
			name: "skip prefix drops the filename prefix",
			opts: CreateOptions{Path: content, TrackerURLs: []string{tracker}, SkipPrefix: true},
			want: "Movie.torrent",
		},
		{
			name: "output path gets .torrent at the end",
			opts: CreateOptions{Path: content, OutputPath: filepath.Join("out", "custom")},
			want: filepath.Join("out", "custom.torrent"),
		},
		{
			name: "output path that ends in .torrent stays as given",
			opts: CreateOptions{Path: content, OutputPath: "custom.torrent"},
			want: "custom.torrent",
		},
		{
			name: "output path gets no filename prefix",
			opts: CreateOptions{Path: content, Name: "Custom", OutputPath: "custom", TrackerURLs: []string{tracker}},
			want: "custom.torrent",
		},
		{
			name: "absolute output path with no output folder stays as given",
			opts: CreateOptions{Path: content, OutputPath: absOut, TrackerURLs: []string{tracker}},
			want: absOut,
		},
		{
			name: "output folder holds the default filename",
			opts: CreateOptions{Path: content, OutputDir: "out", TrackerURLs: []string{tracker}},
			want: filepath.Join("out", "example_Movie.torrent"),
		},
		{
			name: "relative output path goes into the output folder",
			opts: CreateOptions{Path: content, OutputDir: "out", OutputPath: "custom.torrent", TrackerURLs: []string{tracker}},
			want: filepath.Join("out", "custom.torrent"),
		},
		{
			name: "relative output path without extension goes into the output folder",
			opts: CreateOptions{Path: content, OutputDir: "out", OutputPath: filepath.Join("sub", "custom"), TrackerURLs: []string{tracker}},
			want: filepath.Join("out", "sub", "custom.torrent"),
		},
		{
			name: "output folder with trailing separator does not duplicate separator",
			opts: CreateOptions{Path: content, OutputDir: "out" + string(os.PathSeparator), OutputPath: "custom.torrent", TrackerURLs: []string{tracker}},
			want: filepath.Join("out", "custom.torrent"),
		},
		{
			name: "relative output path preserves parent traversal segments",
			opts: CreateOptions{Path: content, OutputDir: "link", OutputPath: filepath.Join("..", "custom"), TrackerURLs: []string{tracker}},
			want: "link" + string(os.PathSeparator) + filepath.Join("..", "custom.torrent"),
		},
		{
			name: "absolute output path wins over the output folder",
			opts: CreateOptions{Path: content, OutputDir: "out", OutputPath: absOut, TrackerURLs: []string{tracker}},
			want: absOut,
		},
		{
			name: "absolute output path without extension wins over the output folder",
			opts: CreateOptions{Path: content, OutputDir: "out", OutputPath: absOutNoExt, TrackerURLs: []string{tracker}},
			want: absOutNoExt + ".torrent",
		},
		{
			name:   "relative output path goes into the preset output folder",
			opts:   CreateOptions{Path: content, OutputPath: "custom.torrent", TrackerURLs: []string{tracker}},
			preset: &preset.Options{OutputDir: "preset-out"},
			want:   filepath.Join("preset-out", "custom.torrent"),
		},
		{
			name:   "absolute output path wins over the preset output folder",
			opts:   CreateOptions{Path: content, OutputPath: absOut, TrackerURLs: []string{tracker}},
			preset: &preset.Options{OutputDir: "preset-out"},
			want:   absOut,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			if tt.preset != nil {
				var err error
				opts, err = ResolveCreateOptions(CreateOverrides{
					CreateSettings: CreateSettings{TrackerURLs: tt.opts.TrackerURLs},
					Path:           tt.opts.Path,
					Name:           tt.opts.Name,
					OutputPath:     tt.opts.OutputPath,
				}, tt.preset)
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, createOutputPath(opts))
		})
	}
}

func TestCreateOutputPathWindowsDriveRelative(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows drive-relative paths apply only on Windows")
	}

	content := filepath.Join("data", "Movie")
	assert.Equal(t, `C:custom.torrent`, createOutputPath(CreateOptions{Path: content, OutputDir: `C:`, OutputPath: "custom"}))
	assert.Equal(t, `C:\custom.torrent`, createOutputPath(CreateOptions{Path: content, OutputDir: `C:\`, OutputPath: "custom"}))
	assert.Equal(t, `\\server\share\custom.torrent`, createOutputPath(CreateOptions{Path: content, OutputDir: `\\server\share`, OutputPath: "custom"}))
}

func TestPlanCreateDoesNotCreateOutputFolderOrFile(t *testing.T) {
	dir := t.TempDir()
	content := filepath.Join(dir, "content.bin")
	require.NoError(t, os.WriteFile(content, make([]byte, 64*1024), 0o644))

	missingDir := filepath.Join(dir, "missing-dir")
	missingPath := filepath.Join(dir, "missing-parent", "nested", "out.torrent")

	tests := []struct {
		name        string
		opts        CreateOptions
		absentPaths []string
	}{
		{
			name:        "missing output dir",
			opts:        CreateOptions{Path: content, OutputDir: missingDir, NoCreator: true, NoDate: true},
			absentPaths: []string{missingDir},
		},
		{
			name:        "missing output path parent",
			opts:        CreateOptions{Path: content, OutputPath: missingPath, NoCreator: true, NoDate: true},
			absentPaths: []string{filepath.Join(dir, "missing-parent"), missingPath},
		},
		{
			name:        "relative output path with missing output dir",
			opts:        CreateOptions{Path: content, OutputDir: missingDir, OutputPath: filepath.Join("sub", "out.torrent"), NoCreator: true, NoDate: true},
			absentPaths: []string{missingDir},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := PlanCreate(tt.opts)
			require.NoError(t, err)
			require.Equal(t, int64(64*1024), plan.ContentSize)
			for _, p := range tt.absentPaths {
				_, statErr := os.Stat(p)
				assert.ErrorIs(t, statErr, os.ErrNotExist)
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
			opts := CreateOptions{
				Path:           tmpDir,
				TrackerURLs:    tt.trackers,
				IsPrivate:      true,
				NoCreator:      true,
				NoDate:         true,
				PieceLengthExp: new(uint(16)),
			}

			tor, err := CreateTorrent(opts)
			if err != nil {
				t.Fatalf("CreateTorrent() failed: %v", err)
			}
			mi := tor.MetaInfo

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
		})
	}
}

func TestCreate_NameArgument(t *testing.T) {
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("create test with -name argument"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "test2.txt"), []byte("create test with -name argument #2"), 0644); err != nil {
		t.Fatalf("Failed to create test file2: %v", err)
	}

	tests := []struct {
		name         string
		opts         CreateOptions
		expectedName string
	}{
		{
			name:         "Single file with no --name argument",
			opts:         CreateOptions{Path: testFile},
			expectedName: "test.txt",
		},
		{
			name:         "Single file with --name argument",
			opts:         CreateOptions{Path: testFile, Name: "customname"},
			expectedName: "customname",
		},
		{
			name:         "Multiple files with no --name argument",
			opts:         CreateOptions{Path: testDir},
			expectedName: filepath.Base(testDir),
		},
		{
			name:         "Multiple files with --name argument",
			opts:         CreateOptions{Path: testDir, Name: "customname"},
			expectedName: "customname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.TrackerURLs = []string{"https://unknown.customtracker.com/announce"}
			tt.opts.Quiet = true

			tor, err := CreateTorrent(tt.opts)
			if err != nil {
				t.Fatalf("CreateTorrent() failed: %v", err)
			}

			if got := tor.GetInfo().Name; got != tt.expectedName {
				t.Fatalf("Expected torrent name %q, got %q", tt.expectedName, got)
			}
		})
	}
}

func TestPlanCreateMatchesCreateAndAppliesFilters(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.mkv")
	skip := filepath.Join(dir, "skip.nfo")
	require.NoError(t, os.WriteFile(keep, make([]byte, 128*1024), 0o644))
	require.NoError(t, os.WriteFile(skip, make([]byte, 512*1024), 0o644))

	hashed := false
	opts := CreateOptions{
		Path:            dir,
		TrackerURLs:     []string{"https://portugas.org/announce"},
		ExcludePatterns: []string{"*.nfo"},
		NoCreator:       true,
		NoDate:          true,
		ProgressCallback: func(completed, total int, hashRate float64) {
			hashed = true
		},
	}

	plan, err := PlanCreate(opts)
	require.NoError(t, err)
	require.Equal(t, int64(128*1024), plan.ContentSize)
	require.Equal(t, uint(14), plan.PieceLengthExponent)
	require.Equal(t, int64(1<<14), plan.PieceLengthBytes)
	require.Equal(t, uint64(2<<20), plan.TrackerSizeLimit)
	require.NotZero(t, plan.PredictedTorrentSize)
	require.False(t, hashed, "planning must not hash content")

	wantExp, notices, err := ChoosePieceLength(opts)
	require.NoError(t, err)
	require.Equal(t, wantExp, plan.PieceLengthExponent)
	require.Equal(t, plan.Notices, notices)

	tor, err := CreateTorrent(opts)
	require.NoError(t, err)
	require.Equal(t, plan.PieceLengthBytes, tor.GetInfo().PieceLength)
}

func TestPlanCreateFailOnSeasonWarningMatchesCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Show.S01")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E01.mkv"), []byte("one"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E03.mkv"), []byte("three"), 0o644))

	opts := CreateOptions{
		Path:                    dir,
		FailOnSeasonPackWarning: true,
		NoCreator:               true,
		NoDate:                  true,
	}
	_, err := PlanCreate(opts)
	require.ErrorContains(t, err, "season pack is suspicious (season 1, missing episodes [2])")

	_, err = CreateTorrent(opts)
	require.ErrorContains(t, err, "season pack is suspicious")

	// without the fail flag, the plan shows the missing episodes before hashing,
	// and it has the same season pack check as create
	opts.FailOnSeasonPackWarning = false
	opts.OutputDir = t.TempDir()
	plan, err := PlanCreate(opts)
	require.NoError(t, err)
	require.Equal(t, &SeasonPackInfo{
		Episodes:        []int{1, 3},
		MissingEpisodes: []int{2},
		Season:          1,
		MaxEpisode:      3,
		VideoFileCount:  2,
		IsSeasonPack:    true,
		IsSuspicious:    true,
	}, plan.SeasonPack)
	info, err := Create(opts)
	require.NoError(t, err)
	require.Equal(t, plan.SeasonPack, info.SeasonPack)

	// the check runs after the exclude patterns: excluding E03 leaves a complete pack
	opts.ExcludePatterns = []string{"*E03*"}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E02.mkv"), []byte("two"), 0o644))
	plan, err = PlanCreate(opts)
	require.NoError(t, err)
	require.Equal(t, &SeasonPackInfo{
		Episodes:        []int{1, 2},
		MissingEpisodes: []int{},
		Season:          1,
		MaxEpisode:      2,
		VideoFileCount:  2,
		IsSeasonPack:    true,
	}, plan.SeasonPack)

	info, err = Create(opts)
	require.NoError(t, err)
	require.Equal(t, plan.SeasonPack, info.SeasonPack)
}

func TestPlanCreateLargeSparseInputDoesNotMaterializePieceHashes(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "large.bin")
	f, err := os.Create(contentPath)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(1<<32))
	require.NoError(t, f.Close())

	maxPieceLength := uint(16)
	hashed := false
	plan, err := PlanCreate(CreateOptions{
		Path:           contentPath,
		MaxPieceLength: &maxPieceLength,
		NoCreator:      true,
		NoDate:         true,
		ProgressCallback: func(completed, total int, hashRate float64) {
			hashed = true
		},
	})
	require.NoError(t, err)
	require.False(t, hashed, "planning must not hash content")
	require.Equal(t, int64(1<<32), plan.ContentSize)
	require.Equal(t, int64(1<<16), plan.PieceLengthBytes)
	require.NotZero(t, plan.PredictedTorrentSize)
}

func TestPrepareCreatePredictsExactInternalPieceLength(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "content.bin")
	require.NoError(t, os.WriteFile(contentPath, make([]byte, 128*1024), 0o644))

	const pieceLength int64 = 65_537
	prepared, err := prepareCreate(CreateOptions{
		Path:      contentPath,
		NoCreator: true,
		NoDate:    true,
	}, createTorrentOptions{pieceLengthBytes: pieceLength})
	require.NoError(t, err)
	require.Equal(t, pieceLength, prepared.pieceLength)
	require.Equal(t, pieceLength, prepared.plan.PieceLengthBytes)

	numPieces, err := pieceCountForSize(prepared.totalSize, pieceLength)
	require.NoError(t, err)
	tor, err := prepared.encode(pieceLength, make([]byte, numPieces*20))
	require.NoError(t, err)
	data, err := bencode.Marshal(tor.MetaInfo)
	require.NoError(t, err)
	require.Equal(t, uint64(len(data)), prepared.plan.PredictedTorrentSize)
}

func Test_choosePieceLength_MaxPieceLengthBounds(t *testing.T) {
	const (
		bhdURL      = "https://beyond-hd.me/announce?passkey=123"
		portugasURL = "https://portugas.org/announce/passkey"
	)
	tests := []struct {
		name        string
		trackerURL  string
		maxExp      uint
		targetCount *uint
		wantErr     bool
	}{
		{"no tracker accepts 32 KiB", "", 15, nil, false},
		{"no tracker rejects 16 KiB", "", 14, nil, true},
		{"no tracker accepts 64 KiB", "", 16, nil, false},
		{"target piece count rejects 32 KiB with no tracker", "", 15, new(uint(1000)), true},
		{"bhd default ranges reject 32 KiB", bhdURL, 15, nil, true},
		{"bhd default ranges accept 64 KiB", bhdURL, 16, nil, false},
		{"portugas custom table accepts 16 KiB", portugasURL, 14, nil, false},
		{"portugas custom table rejects 8 KiB", portugasURL, 13, nil, true},
		{"target piece count rejects 32 KiB on a custom table", portugasURL, 15, new(uint(1000)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var trackerURLs []string
			if tt.trackerURL != "" {
				trackerURLs = []string{tt.trackerURL}
			}
			got, _, err := choosePieceLength(1<<20, CreateOptions{
				TrackerURLs:      trackerURLs,
				MaxPieceLength:   new(tt.maxExp),
				TargetPieceCount: tt.targetCount,
			}, rulesFor(trackerURLs), nil)
			if tt.wantErr {
				require.ErrorContains(t, err, "max piece length exponent must be between")
				if tt.trackerURL != "" {
					require.ErrorContains(t, err, " for "+tt.trackerURL)
				}
				return
			}
			require.NoError(t, err)
			assert.LessOrEqual(t, got, tt.maxExp)
		})
	}
}

func TestCreateResultReportsSizeFilesPiecesAndSeasonPack(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "single.bin")
		require.NoError(t, os.WriteFile(path, make([]byte, 150*1024), 0o644))

		info, err := Create(CreateOptions{
			Path:           path,
			OutputDir:      filepath.Join(dir, "out"),
			TrackerURLs:    []string{"https://tracker1.example.invalid/announce", "https://tracker2.example.invalid/announce"},
			PieceLengthExp: new(uint(16)), // 64 KiB pieces
			NoCreator:      true,
			NoDate:         true,
			Quiet:          true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(150*1024), info.Size)
		require.Equal(t, 1, info.Files)
		require.Equal(t, 3, info.PieceCount)
		require.Equal(t, "https://tracker1.example.invalid/announce", info.Announce)
		require.False(t, info.SeasonPack.IsSeasonPack)
	})

	t.Run("folder with an excluded episode", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "Show.S01.1080p")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E01.mkv"), make([]byte, 100*1024), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E03.mkv"), make([]byte, 100*1024), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Show.S01E05.mkv"), make([]byte, 100*1024), 0o644))

		info, err := Create(CreateOptions{
			Path:            dir,
			OutputDir:       filepath.Join(root, "out"),
			ExcludePatterns: []string{"*E05*"},
			PieceLengthExp:  new(uint(16)),
			NoCreator:       true,
			NoDate:          true,
			Quiet:           true,
		})
		require.NoError(t, err)
		require.Equal(t, int64(200*1024), info.Size)
		require.Equal(t, 2, info.Files)
		require.Equal(t, 4, info.PieceCount)
		require.Empty(t, info.Announce)
		// the excluded E05 is not in the torrent, so only E02 is missing
		require.Equal(t, &SeasonPackInfo{
			Episodes:        []int{1, 3},
			MissingEpisodes: []int{2},
			Season:          1,
			MaxEpisode:      3,
			VideoFileCount:  2,
			IsSeasonPack:    true,
			IsSuspicious:    true,
		}, info.SeasonPack)
	})
}

// Test_choosePieceLength_AllowedPieceLengths checks -l and -m against the
// allowed piece lengths for every tracker rule set. The GUI and batch read the
// same range, so a gap here is a value that they offer and create rejects.
func Test_choosePieceLength_AllowedPieceLengths(t *testing.T) {
	ruleSets := append([]trackers.Rules{{}}, trackers.Configs()...)
	for _, rules := range ruleSets {
		name := "no tracker"
		var trackerURLs []string
		if len(rules.URLs) > 0 {
			name = rules.URLs[0]
			trackerURLs = []string{"https://" + rules.URLs[0] + "/announce"}
		}
		t.Run(name, func(t *testing.T) {
			for exp := uint(10); exp <= 28; exp++ {
				cases := []struct {
					label       string
					opts        CreateOptions
					targetCount bool
				}{
					{"-l", CreateOptions{PieceLengthExp: new(exp)}, false},
					{"-m", CreateOptions{MaxPieceLength: new(exp)}, false},
					{"-m with target piece count", CreateOptions{MaxPieceLength: new(exp), TargetPieceCount: new(uint(100))}, true},
				}
				for _, c := range cases {
					c.opts.TrackerURLs = trackerURLs
					lower, upper := trackers.AllowedPieceLengths(rules, c.targetCount)
					_, _, err := choosePieceLength(1<<20, c.opts, rules, nil)
					if exp >= lower && exp <= upper {
						assert.NoError(t, err, "%s %d", c.label, exp)
					} else {
						assert.ErrorContains(t, err, "exponent must be between", "%s %d", c.label, exp)
					}
				}
			}
		})
	}
}
