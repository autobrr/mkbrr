package torrent

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// TestUpdateTorrentRenameAndAppendReusesPieces verifies mixed hash reuse and boundary rehashing against a clean rebuild.
func TestUpdateTorrentRenameAndAppendReusesPieces(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 70_000))
	writeUpdateTestFile(t, filepath.Join(contentDir, "m.bin"), bytes.Repeat([]byte{'m'}, 70_001))

	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		Name:           "release",
		PieceLengthExp: &pieceLength,
		Source:         "source-tag",
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	original.Comment = "keep-comment"
	addUpdateTestInfoValue(t, original.MetaInfo, "custom-key", "keep-value")

	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	if err := os.Rename(filepath.Join(contentDir, "a.bin"), filepath.Join(contentDir, "b.bin")); err != nil {
		t.Fatalf("Rename(a.bin, b.bin) error: %v", err)
	}
	writeUpdateTestFile(t, filepath.Join(contentDir, "z.bin"), bytes.Repeat([]byte{'z'}, 20_000))

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.ReusedPieces, 2; got != want {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want %d", got, want)
	}
	if got, want := result.HashedPieces, 1; got != want {
		t.Errorf("UpdateTorrent().HashedPieces = %d, want %d", got, want)
	}

	updated, err := metainfo.LoadFromFile(torrentPath)
	if err != nil {
		t.Fatalf("LoadFromFile(updated) error: %v", err)
	}
	updatedInfo, err := updated.UnmarshalInfo()
	if err != nil {
		t.Fatalf("UnmarshalInfo(updated) error: %v", err)
	}

	fullyHashed, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		Name:           "release",
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(fully hashed) error: %v", err)
	}
	fullyHashedInfo, err := fullyHashed.UnmarshalInfo()
	if err != nil {
		t.Fatalf("UnmarshalInfo(fully hashed) error: %v", err)
	}

	if !bytes.Equal(updatedInfo.Pieces, fullyHashedInfo.Pieces) {
		t.Error("UpdateTorrent() piece hashes differ from a full rehash")
	}
	if got, want := updateTestPaths(updatedInfo.Files), []string{"b.bin", "m.bin", "z.bin"}; !equalUpdateTestStrings(got, want) {
		t.Errorf("UpdateTorrent() paths = %v, want %v", got, want)
	}
	if got, want := updated.Comment, "keep-comment"; got != want {
		t.Errorf("UpdateTorrent() comment = %q, want %q", got, want)
	}

	infoMap := make(map[string]any)
	if err := bencode.Unmarshal(updated.InfoBytes, &infoMap); err != nil {
		t.Fatalf("Unmarshal(updated.InfoBytes) error: %v", err)
	}
	if got, want := infoMap["custom-key"], "keep-value"; got != want {
		t.Errorf("UpdateTorrent() custom-key = %v, want %v", got, want)
	}
	if got, want := infoMap["source"], "source-tag"; got != want {
		t.Errorf("UpdateTorrent() source = %v, want %v", got, want)
	}
}

// TestUpdateTorrentAmbiguousRenamesRequireMapping verifies that equal-size rename ambiguity is never guessed.
func TestUpdateTorrentAmbiguousRenamesRequireMapping(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 65_536))
	writeUpdateTestFile(t, filepath.Join(contentDir, "b.bin"), bytes.Repeat([]byte{'b'}, 65_536))

	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		Name:           "release",
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	if err := os.Rename(filepath.Join(contentDir, "a.bin"), filepath.Join(contentDir, "c.bin")); err != nil {
		t.Fatalf("Rename(a.bin, c.bin) error: %v", err)
	}
	if err := os.Rename(filepath.Join(contentDir, "b.bin"), filepath.Join(contentDir, "d.bin")); err != nil {
		t.Fatalf("Rename(b.bin, d.bin) error: %v", err)
	}

	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "--rename") {
		t.Fatalf("UpdateTorrent(ambiguous rename) error = %v, want --rename guidance", err)
	}

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			"a.bin": "c.bin",
			"b.bin": "d.bin",
		},
		Quiet: true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent(explicit renames) error: %v", err)
	}
	if got, want := result.ReusedPieces, 2; got != want {
		t.Errorf("UpdateTorrent(explicit renames).ReusedPieces = %d, want %d", got, want)
	}
	if got := result.HashedPieces; got != 0 {
		t.Errorf("UpdateTorrent(explicit renames).HashedPieces = %d, want 0", got)
	}
}

// writeUpdateTestFile creates content fixtures with deterministic bytes.
func writeUpdateTestFile(t *testing.T, filePath string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", filePath, err)
	}
}

// writeUpdateTestTorrent serializes metainfo for update tests.
func writeUpdateTestTorrent(t *testing.T, torrentPath string, mi *metainfo.MetaInfo) {
	t.Helper()
	file, err := os.Create(torrentPath)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", torrentPath, err)
	}
	if err := mi.Write(file); err != nil {
		_ = file.Close()
		t.Fatalf("MetaInfo.Write(%q) error: %v", torrentPath, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%q) error: %v", torrentPath, err)
	}
}

// addUpdateTestInfoValue injects a custom info key to verify lossless metadata preservation.
func addUpdateTestInfoValue(t *testing.T, mi *metainfo.MetaInfo, key string, value any) {
	t.Helper()
	infoMap := make(map[string]any)
	if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
		t.Fatalf("Unmarshal(InfoBytes) error: %v", err)
	}
	infoMap[key] = value
	infoBytes, err := bencode.Marshal(infoMap)
	if err != nil {
		t.Fatalf("Marshal(infoMap) error: %v", err)
	}
	mi.InfoBytes = infoBytes
}

// updateTestPaths flattens metainfo path components for readable comparisons.
func updateTestPaths(files []metainfo.FileInfo) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = strings.Join(file.Path, "/")
	}
	return paths
}

// equalUpdateTestStrings compares ordered path lists without introducing an assertion dependency.
func equalUpdateTestStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
