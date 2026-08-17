package torrent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
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
		Renames: map[string]string{
			"a.bin": "b.bin",
		},
		Quiet:   true,
		InPlace: true,
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

// TestUpdateTorrentPrefersExactPaths prevents normalized aliases from stealing an exact match.
func TestUpdateTorrentPrefersExactPaths(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, " a.bin"), bytes.Repeat([]byte{'a'}, 65_536))
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'b'}, 65_536))

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

	if err := os.Remove(filepath.Join(contentDir, " a.bin")); err != nil {
		t.Fatalf("Remove(space-prefixed file) error: %v", err)
	}
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.ReusedPieces, 1; got != want {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want %d", got, want)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"a.bin"})
}

// TestUpdateTorrentPreservesRootKeys verifies known and unknown root metadata survives.
func TestUpdateTorrentPreservesRootKeys(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 65_536))

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

	rootValues := map[string]any{
		"httpseeds":    []string{"https://seed.example/content"},
		"url-list":     []string{"https://webseed.example/content"},
		"x_cross_seed": "keep-me",
	}
	addUpdateTestRootValues(t, torrentPath, rootValues)

	if _, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	}); err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}

	rootMap := readUpdateTestRoot(t, torrentPath)
	for key, value := range rootValues {
		want, err := bencode.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal(root value %q) error: %v", key, err)
		}
		if got := rootMap[key]; !bytes.Equal(got, want) {
			t.Errorf("UpdateTorrent() root key %q = %q, want %q", key, got, want)
		}
	}
}

// TestUpdateTorrentPreservesRawInfoValues verifies arbitrary valid bencode survives without int64 coercion.
func TestUpdateTorrentPreservesRawInfoValues(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 65_536))

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

	bigInteger := bencode.Bytes("i9223372036854775808e")
	addUpdateTestRawInfoValues(t, torrentPath,
		map[string]bencode.Bytes{"x-bigint": bigInteger},
		0,
		map[string]bencode.Bytes{"x-file-bigint": bigInteger},
	)
	if _, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	}); err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}

	infoMap := readUpdateTestRawInfo(t, torrentPath)
	if got := infoMap["x-bigint"]; !bytes.Equal(got, bigInteger) {
		t.Errorf("UpdateTorrent() x-bigint = %q, want %q", got, bigInteger)
	}
	var files []map[string]bencode.Bytes
	if err := bencode.Unmarshal(infoMap["files"], &files); err != nil {
		t.Fatalf("Unmarshal(files) error: %v", err)
	}
	if got := files[0]["x-file-bigint"]; !bytes.Equal(got, bigInteger) {
		t.Errorf("UpdateTorrent() x-file-bigint = %q, want %q", got, bigInteger)
	}
}

// TestUpdateTorrentRejectsUnsafeOutputPaths verifies output cannot replace or enter the content set.
func TestUpdateTorrentRejectsUnsafeOutputPaths(t *testing.T) {
	t.Run("content file identities", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			setup func(t *testing.T, contentPath string) string
		}{
			{
				name: "same path",
				setup: func(_ *testing.T, contentPath string) string {
					return contentPath
				},
			},
			{
				name: "hard link",
				setup: func(t *testing.T, contentPath string) string {
					outputPath := filepath.Join(t.TempDir(), "hard-link.torrent")
					if err := os.Link(contentPath, outputPath); err != nil {
						t.Skipf("hard links unavailable: %v", err)
					}
					return outputPath
				},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				contentPath := filepath.Join(t.TempDir(), "content.bin")
				contentBytes := []byte("original content bytes")
				writeUpdateTestFile(t, contentPath, contentBytes)
				pieceLength := uint(16)
				original, err := CreateTorrent(CreateOptions{
					Path:           contentPath,
					PieceLengthExp: &pieceLength,
					Quiet:          true,
				})
				if err != nil {
					t.Fatalf("CreateTorrent(original) error: %v", err)
				}
				torrentPath := filepath.Join(t.TempDir(), "input.torrent")
				writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

				_, err = UpdateTorrent(UpdateOptions{
					TorrentPath: torrentPath,
					ContentPath: contentPath,
					OutputPath:  test.setup(t, contentPath),
					Quiet:       true,
				})
				if err == nil || !strings.Contains(err.Error(), "must not replace the content file") {
					t.Fatalf("UpdateTorrent() error = %v, want content output refusal", err)
				}
				got, err := os.ReadFile(contentPath)
				if err != nil {
					t.Fatalf("ReadFile(content) error: %v", err)
				}
				if !bytes.Equal(got, contentBytes) {
					t.Error("UpdateTorrent() changed content after refusing output path")
				}
			})
		}
	})

	t.Run("inside content directory", func(t *testing.T) {
		contentDir := t.TempDir()
		writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), []byte("content"))
		pieceLength := uint(16)
		original, err := CreateTorrent(CreateOptions{
			Path:           contentDir,
			PieceLengthExp: &pieceLength,
			Quiet:          true,
		})
		if err != nil {
			t.Fatalf("CreateTorrent(original) error: %v", err)
		}
		torrentPath := filepath.Join(t.TempDir(), "input.torrent")
		writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
		outputPath := filepath.Join(contentDir, "nested", "output.data")

		_, err = UpdateTorrent(UpdateOptions{
			TorrentPath: torrentPath,
			ContentPath: contentDir,
			OutputPath:  outputPath,
			Quiet:       true,
		})
		if err == nil || !strings.Contains(err.Error(), "must not be inside the content directory") {
			t.Fatalf("UpdateTorrent() error = %v, want nested output refusal", err)
		}
		if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
			t.Fatalf("Stat(output) error = %v, want nonexistent output", err)
		}
	})
}

func TestUpdateTorrentReplacesOutputSymlinkWithoutFollowingTarget(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "content.bin")
	contentBytes := bytes.Repeat([]byte{'a'}, 65_536)
	writeUpdateTestFile(t, contentPath, contentBytes)
	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentPath,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "input.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	outputPath := filepath.Join(t.TempDir(), "output.torrent")
	if err := os.Symlink(contentPath, outputPath); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}

	if _, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		OutputPath:  outputPath,
		Quiet:       true,
	}); err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	gotContent, err := os.ReadFile(contentPath)
	if err != nil {
		t.Fatalf("ReadFile(content) error: %v", err)
	}
	if !bytes.Equal(gotContent, contentBytes) {
		t.Error("UpdateTorrent() followed the output symlink and replaced content")
	}
	outputInfo, err := os.Lstat(outputPath)
	if err != nil {
		t.Fatalf("Lstat(output) error: %v", err)
	}
	if outputInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("UpdateTorrent() did not atomically replace the output symlink")
	}
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: outputPath,
		ContentPath: contentPath,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData() error: %v", err)
	}
	if verification.Completion != 100 {
		t.Errorf("VerifyData().Completion = %.2f, want 100", verification.Completion)
	}
}

func TestUpdateTorrentRejectsEquivalentContentDirectorySpellings(t *testing.T) {
	for _, test := range []struct {
		name          string
		directoryName string
		aliasName     string
	}{
		{name: "case variant", directoryName: "content-case", aliasName: "CONTENT-CASE"},
		{name: "Unicode variant", directoryName: "caf\u00e9", aliasName: "cafe\u0301"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			contentDir := filepath.Join(parent, test.directoryName)
			if err := os.Mkdir(contentDir, 0o755); err != nil {
				t.Fatalf("Mkdir(content) error: %v", err)
			}
			contentAlias := filepath.Join(parent, test.aliasName)
			contentInfo, err := os.Stat(contentDir)
			if err != nil {
				t.Fatalf("Stat(content) error: %v", err)
			}
			aliasInfo, err := os.Stat(contentAlias)
			if err != nil || !os.SameFile(contentInfo, aliasInfo) {
				t.Skip("filesystem treats the alternate spelling as a different path")
			}
			writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), []byte("content"))
			pieceLength := uint(16)
			original, err := CreateTorrent(CreateOptions{
				Path:           contentDir,
				PieceLengthExp: &pieceLength,
				Quiet:          true,
			})
			if err != nil {
				t.Fatalf("CreateTorrent(original) error: %v", err)
			}
			torrentPath := filepath.Join(t.TempDir(), "input.torrent")
			writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
			outputPath := filepath.Join(contentAlias, "output.torrent")

			_, err = UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: contentDir,
				OutputPath:  outputPath,
				Quiet:       true,
			})
			if err == nil || !strings.Contains(err.Error(), "inside the content directory") {
				t.Fatalf("UpdateTorrent() error = %v, want equivalent-path containment refusal", err)
			}
			if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
				t.Fatalf("Stat(output) error = %v, want nonexistent output", err)
			}
		})
	}
}

func TestUpdateTorrentRefusesChangedOutputParent(t *testing.T) {
	contentDir := t.TempDir()
	contentPath := filepath.Join(contentDir, "victim.bin")
	contentBytes := bytes.Repeat([]byte{'a'}, 131_072)
	writeUpdateTestFile(t, contentPath, contentBytes)

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
	torrentPath := filepath.Join(t.TempDir(), "input.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	safeOutputDir := t.TempDir()
	outputParent := filepath.Join(t.TempDir(), "output-parent")
	if err := os.Symlink(safeOutputDir, outputParent); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	outputPath := filepath.Join(outputParent, "victim.bin")
	swapped := false
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		OutputPath:  outputPath,
		Quiet:       true,
		ProgressCallback: func(_, _ int, _ float64) {
			if swapped {
				return
			}
			swapped = true
			if removeErr := os.Remove(outputParent); removeErr != nil {
				t.Fatalf("Remove(output parent symlink) error: %v", removeErr)
			}
			if symlinkErr := os.Symlink(contentDir, outputParent); symlinkErr != nil {
				t.Fatalf("Symlink(swapped output parent) error: %v", symlinkErr)
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "output parent changed during update") {
		t.Fatalf("UpdateTorrent() error = %v, want changed-parent refusal", err)
	}
	gotContent, err := os.ReadFile(contentPath)
	if err != nil {
		t.Fatalf("ReadFile(content) error: %v", err)
	}
	if !bytes.Equal(gotContent, contentBytes) {
		t.Error("UpdateTorrent() replaced content after output parent changed")
	}
	if _, err := os.Stat(filepath.Join(safeOutputDir, "victim.bin")); !os.IsNotExist(err) {
		t.Fatalf("Stat(safe output) error = %v, want nonexistent output", err)
	}
}

// TestUpdateTorrentRejectsNegativeFileLengths verifies malformed offsets cannot reuse stale hashes.
func TestUpdateTorrentRejectsNegativeFileLengths(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), []byte("a"))
	writeUpdateTestFile(t, filepath.Join(contentDir, "b.bin"), []byte("b"))
	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	addUpdateTestFileValues(t, original.MetaInfo, 0, map[string]any{"length": int64(-1)})
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	before, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent) error: %v", err)
	}

	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "negative length") {
		t.Fatalf("UpdateTorrent() error = %v, want negative-length rejection", err)
	}
	after, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent after refusal) error: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Error("UpdateTorrent() changed malformed torrent after refusing it")
	}
}

func TestUpdateTorrentRejectsConflictingFileLayouts(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), []byte("a"))
	writeUpdateTestFile(t, filepath.Join(contentDir, "b.bin"), []byte("b"))
	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	addUpdateTestRawInfoValues(t, torrentPath, map[string]bencode.Bytes{
		"length": bencode.Bytes("i0e"),
	}, 0, nil)
	before, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent) error: %v", err)
	}

	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "both files and length") {
		t.Fatalf("UpdateTorrent() error = %v, want conflicting-layout rejection", err)
	}
	after, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent after refusal) error: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Error("UpdateTorrent() changed malformed torrent after refusing conflicting layout")
	}
}

func TestUpdateTorrentRejectsInvalidMultiFileName(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), []byte("a"))
	writeUpdateTestFile(t, filepath.Join(contentDir, "b.bin"), []byte("b"))
	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	addUpdateTestRawInfoValues(t, torrentPath, map[string]bencode.Bytes{
		"name": bencode.Bytes("0:"),
	}, 0, nil)
	before, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent) error: %v", err)
	}

	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid name") {
		t.Fatalf("UpdateTorrent() error = %v, want invalid-name rejection", err)
	}
	after, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(torrent after refusal) error: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Error("UpdateTorrent() changed malformed torrent after refusing invalid name")
	}
}

// TestUpdateTorrentHandlesExtremePieceLength verifies ceil division cannot overflow into a panic.
func TestUpdateTorrentHandlesExtremePieceLength(t *testing.T) {
	contentPath := filepath.Join(t.TempDir(), "content.bin")
	writeUpdateTestFile(t, contentPath, []byte("ab"))
	infoBytes, err := bencode.Marshal(metainfo.Info{
		Name:        "content.bin",
		PieceLength: maxTorrentDataSize,
		Length:      1,
		Pieces:      make([]byte, 20),
	})
	if err != nil {
		t.Fatalf("Marshal(info) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "extreme.torrent")
	writeUpdateTestTorrent(t, torrentPath, &metainfo.MetaInfo{InfoBytes: infoBytes})

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if result.TotalPieces != 1 || result.HashedPieces != 1 {
		t.Errorf("UpdateTorrent() pieces = total %d, hashed %d; want 1, 1", result.TotalPieces, result.HashedPieces)
	}
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData() error: %v", err)
	}
	if verification.Completion != 100 {
		t.Errorf("VerifyData().Completion = %.2f, want 100", verification.Completion)
	}
}

// TestUpdateTorrentReusesSymlinkAliases verifies torrent-visible aliases remain distinct.
func TestUpdateTorrentReusesSymlinkAliases(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "real.bin"), bytes.Repeat([]byte{'a'}, 131_072))
	for _, name := range []string{"link1.bin", "link2.bin"} {
		if err := os.Symlink("real.bin", filepath.Join(contentDir, name)); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
	}

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
	originalInfo, err := original.UnmarshalInfo()
	if err != nil {
		t.Fatalf("UnmarshalInfo(original) error: %v", err)
	}
	wantPaths := []string{"link1.bin", "link2.bin", "real.bin"}
	if got := updateTestPaths(originalInfo.Files); !equalUpdateTestStrings(got, wantPaths) {
		t.Fatalf("CreateTorrent() paths = %v, want %v", got, wantPaths)
	}

	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if result.ReusedPieces != result.TotalPieces {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want all %d", result.ReusedPieces, result.TotalPieces)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, wantPaths)
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData() error: %v", err)
	}
	if verification.Completion != 100 {
		t.Errorf("VerifyData().Completion = %.2f, want 100", verification.Completion)
	}
}

func TestUpdateTorrentSupportsSymlinkedContentRoot(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 131_072))
	contentAlias := filepath.Join(t.TempDir(), "content-alias")
	if err := os.Symlink(contentDir, contentAlias); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}

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

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentAlias,
		InPlace:     true,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if result.ReusedPieces != result.TotalPieces {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want all %d", result.ReusedPieces, result.TotalPieces)
	}
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentAlias,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("VerifyData() error: %v", err)
	}
	if verification.Completion != 100 {
		t.Errorf("VerifyData().Completion = %.2f, want 100", verification.Completion)
	}
}

// TestUpdateTorrentDefaultsToSeparateOutput verifies the input is untouched without --in-place.
func TestUpdateTorrentDefaultsToSeparateOutput(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 131_072))

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
	originalBytes, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(original torrent) error: %v", err)
	}

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.OutputPath, filepath.Join(filepath.Dir(torrentPath), "release.updated.torrent"); got != want {
		t.Errorf("UpdateTorrent().OutputPath = %q, want %q", got, want)
	}
	afterBytes, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(input torrent) error: %v", err)
	}
	if !bytes.Equal(afterBytes, originalBytes) {
		t.Error("UpdateTorrent() changed the input torrent without InPlace")
	}
	assertUpdateMatchesFullRehash(t, result.OutputPath, contentDir, "release", pieceLength, []string{"a.bin"})
	outputBytes, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("ReadFile(default output) error: %v", err)
	}
	if _, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	}); err == nil || !strings.Contains(err.Error(), "default output") {
		t.Fatalf("UpdateTorrent(existing default output) error = %v, want refusal", err)
	}
	afterRefusal, err := os.ReadFile(result.OutputPath)
	if err != nil {
		t.Fatalf("ReadFile(default output after refusal) error: %v", err)
	}
	if !bytes.Equal(afterRefusal, outputBytes) {
		t.Error("UpdateTorrent() replaced an existing default output")
	}
}

// TestUpdateTorrentRequiresForceForZeroReuse verifies a wrong content path cannot replace the input by default.
func TestUpdateTorrentRequiresForceForZeroReuse(t *testing.T) {
	for _, test := range []struct {
		name          string
		unrelatedSize int
		updatedPieces int
	}{
		{name: "multi-piece replacement", unrelatedSize: 196_608, updatedPieces: 3},
		{name: "single-piece replacement", unrelatedSize: 32_768, updatedPieces: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			originalDir := t.TempDir()
			unrelatedDir := t.TempDir()
			writeUpdateTestFile(t, filepath.Join(originalDir, "original.bin"), bytes.Repeat([]byte{'a'}, 196_608))
			writeUpdateTestFile(t, filepath.Join(unrelatedDir, "unrelated.bin"), bytes.Repeat([]byte{'b'}, test.unrelatedSize))

			pieceLength := uint(16)
			original, err := CreateTorrent(CreateOptions{
				Path:           originalDir,
				Name:           "release",
				PieceLengthExp: &pieceLength,
				Quiet:          true,
			})
			if err != nil {
				t.Fatalf("CreateTorrent(original) error: %v", err)
			}
			torrentPath := filepath.Join(t.TempDir(), "release.torrent")
			writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
			originalBytes, err := os.ReadFile(torrentPath)
			if err != nil {
				t.Fatalf("ReadFile(original torrent) error: %v", err)
			}

			_, err = UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: unrelatedDir,
				InPlace:     true,
				Quiet:       true,
			})
			wantError := fmt.Sprintf("0 reusable pieces (existing torrent: 3, updated torrent: %d)", test.updatedPieces)
			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("UpdateTorrent() error = %v, want %q", err, wantError)
			}
			afterRefusal, err := os.ReadFile(torrentPath)
			if err != nil {
				t.Fatalf("ReadFile(refused torrent) error: %v", err)
			}
			if !bytes.Equal(afterRefusal, originalBytes) {
				t.Error("UpdateTorrent() changed the input torrent after refusing zero reuse")
			}

			result, err := UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: unrelatedDir,
				InPlace:     true,
				Force:       true,
				Quiet:       true,
			})
			if err != nil {
				t.Fatalf("UpdateTorrent(Force) error: %v", err)
			}
			if got := result.ReusedPieces; got != 0 {
				t.Errorf("UpdateTorrent(Force).ReusedPieces = %d, want 0", got)
			}
			assertUpdateMatchesFullRehash(t, torrentPath, unrelatedDir, "release", pieceLength, []string{"unrelated.bin"})
		})
	}
}

// TestUpdateTorrentAmbiguousRenamesFallbackToHashing verifies ambiguity never blocks a valid update.
func TestUpdateTorrentAmbiguousRenamesFallbackToHashing(t *testing.T) {
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
	fallbackTorrentPath := filepath.Join(t.TempDir(), "fallback.torrent")
	explicitTorrentPath := filepath.Join(t.TempDir(), "explicit.torrent")
	writeUpdateTestTorrent(t, fallbackTorrentPath, original.MetaInfo)
	writeUpdateTestTorrent(t, explicitTorrentPath, original.MetaInfo)

	if err := os.Rename(filepath.Join(contentDir, "a.bin"), filepath.Join(contentDir, "c.bin")); err != nil {
		t.Fatalf("Rename(a.bin, c.bin) error: %v", err)
	}
	if err := os.Rename(filepath.Join(contentDir, "b.bin"), filepath.Join(contentDir, "d.bin")); err != nil {
		t.Fatalf("Rename(b.bin, d.bin) error: %v", err)
	}

	fallbackResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: fallbackTorrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent(ambiguous rename) error: %v", err)
	}
	if got := fallbackResult.ReusedPieces; got != 0 {
		t.Errorf("UpdateTorrent(ambiguous rename).ReusedPieces = %d, want 0", got)
	}
	if got, want := fallbackResult.HashedPieces, 2; got != want {
		t.Errorf("UpdateTorrent(ambiguous rename).HashedPieces = %d, want %d", got, want)
	}
	assertUpdateMatchesFullRehash(t, fallbackTorrentPath, contentDir, "release", pieceLength, []string{"c.bin", "d.bin"})

	explicitResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: explicitTorrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			"a.bin": "c.bin",
			"b.bin": "d.bin",
		},
		Quiet:   true,
		InPlace: true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent(explicit renames) error: %v", err)
	}
	if got, want := explicitResult.ReusedPieces, 2; got != want {
		t.Errorf("UpdateTorrent(explicit renames).ReusedPieces = %d, want %d", got, want)
	}
	if got := explicitResult.HashedPieces; got != 0 {
		t.Errorf("UpdateTorrent(explicit renames).HashedPieces = %d, want 0", got)
	}
	assertUpdateMatchesFullRehash(t, explicitTorrentPath, contentDir, "release", pieceLength, []string{"c.bin", "d.bin"})
}

// TestUpdateTorrentSameSizeReplacementRehashes verifies size alone never inherits stale hashes.
func TestUpdateTorrentSameSizeReplacementRehashes(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "old.bin"), bytes.Repeat([]byte{'a'}, 65_536))

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

	if err := os.Remove(filepath.Join(contentDir, "old.bin")); err != nil {
		t.Fatalf("Remove(old.bin) error: %v", err)
	}
	writeUpdateTestFile(t, filepath.Join(contentDir, "new.bin"), bytes.Repeat([]byte{'b'}, 65_536))

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got := result.ReusedPieces; got != 0 {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want 0", got)
	}
	if got, want := result.HashedPieces, 1; got != want {
		t.Errorf("UpdateTorrent().HashedPieces = %d, want %d", got, want)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"new.bin"})
}

// TestUpdateTorrentRemovesZeroLengthFiles verifies metadata-only removals preserve every piece hash.
func TestUpdateTorrentRemovesZeroLengthFiles(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "volume1.cbz"), bytes.Repeat([]byte{'a'}, 70_000))
	removedPaths := []string{
		"volume1.cbz.par2",
		"volume1.cbz.vol00+01.par2",
		"volume1.cbz.vol01+02.par2",
	}
	for _, filePath := range removedPaths {
		writeUpdateTestFile(t, filepath.Join(contentDir, filePath), nil)
	}
	writeUpdateTestFile(t, filepath.Join(contentDir, "volume2.cbz"), bytes.Repeat([]byte{'b'}, 71_001))

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

	for _, filePath := range removedPaths {
		if err := os.Remove(filepath.Join(contentDir, filePath)); err != nil {
			t.Fatalf("Remove(%q) error: %v", filePath, err)
		}
	}

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.ReusedPieces, result.TotalPieces; got != want {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want all %d pieces reused", got, want)
	}
	if got := result.HashedPieces; got != 0 {
		t.Errorf("UpdateTorrent().HashedPieces = %d, want 0", got)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"volume1.cbz", "volume2.cbz"})
}

// TestUpdateTorrentRemovesNonEmptyFile verifies shifted piece boundaries are rehashed correctly.
func TestUpdateTorrentRemovesNonEmptyFile(t *testing.T) {
	contentDir := t.TempDir()
	writeUpdateTestFile(t, filepath.Join(contentDir, "a.bin"), bytes.Repeat([]byte{'a'}, 65_536))
	writeUpdateTestFile(t, filepath.Join(contentDir, "b.bin"), bytes.Repeat([]byte{'b'}, 1_000))
	writeUpdateTestFile(t, filepath.Join(contentDir, "c.bin"), bytes.Repeat([]byte{'c'}, 65_536))

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

	if err := os.Remove(filepath.Join(contentDir, "b.bin")); err != nil {
		t.Fatalf("Remove(b.bin) error: %v", err)
	}
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.ReusedPieces, 1; got != want {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want %d", got, want)
	}
	if got, want := result.HashedPieces, 1; got != want {
		t.Errorf("UpdateTorrent().HashedPieces = %d, want %d", got, want)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"a.bin", "c.bin"})
}

// TestUpdateTorrentSingleFileSameSizeReplacementRehashes verifies basenames gate single-file reuse.
func TestUpdateTorrentSingleFileSameSizeReplacementRehashes(t *testing.T) {
	originalPath := filepath.Join(t.TempDir(), "original.bin")
	replacementPath := filepath.Join(t.TempDir(), "replacement.bin")
	writeUpdateTestFile(t, originalPath, bytes.Repeat([]byte{'a'}, 131_073))
	writeUpdateTestFile(t, replacementPath, bytes.Repeat([]byte{'b'}, 131_073))

	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           originalPath,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	if err != nil {
		t.Fatalf("CreateTorrent(original) error: %v", err)
	}
	torrentPath := filepath.Join(t.TempDir(), "original.torrent")
	outputPath := filepath.Join(t.TempDir(), "updated.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: replacementPath,
		OutputPath:  outputPath,
		Quiet:       true,
		Force:       true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got := result.ReusedPieces; got != 0 {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want 0", got)
	}
	if got, want := result.HashedPieces, result.TotalPieces; got != want {
		t.Errorf("UpdateTorrent().HashedPieces = %d, want all %d pieces hashed", got, want)
	}
	assertUpdateMatchesFullRehash(t, outputPath, replacementPath, "original.bin", pieceLength, nil)

	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat(output) error: %v", err)
	}
	if got, want := outputInfo.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Errorf("UpdateTorrent() output mode = %o, want %o", got, want)
	}
}

// TestUpdateTorrentNormalizesUnicodePathsAndRenames verifies NFC keys match decomposed disk names.
func TestUpdateTorrentNormalizesUnicodePathsAndRenames(t *testing.T) {
	contentDir := t.TempDir()
	oldDiskName := "cafe\u0301.bin"
	oldTorrentName := "caf\u00e9.bin"
	writeUpdateTestFile(t, filepath.Join(contentDir, oldDiskName), bytes.Repeat([]byte{'a'}, 65_536))

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
	addUpdateTestFileValues(t, original.MetaInfo, 0, map[string]any{
		"path": []string{oldTorrentName},
	})

	unchangedTorrentPath := filepath.Join(t.TempDir(), "unchanged.torrent")
	renamedTorrentPath := filepath.Join(t.TempDir(), "renamed.torrent")
	writeUpdateTestTorrent(t, unchangedTorrentPath, original.MetaInfo)
	writeUpdateTestTorrent(t, renamedTorrentPath, original.MetaInfo)

	unchangedResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: unchangedTorrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent(unchanged NFD path) error: %v", err)
	}
	if got, want := unchangedResult.ReusedPieces, unchangedResult.TotalPieces; got != want {
		t.Errorf("UpdateTorrent(unchanged NFD path).ReusedPieces = %d, want all %d pieces reused", got, want)
	}
	assertUpdateMatchesFullRehash(t, unchangedTorrentPath, contentDir, "release", pieceLength, nil)

	newDiskName := "renome\u0301.bin"
	newTorrentName := "renom\u00e9.bin"
	if err := os.Rename(filepath.Join(contentDir, oldDiskName), filepath.Join(contentDir, newDiskName)); err != nil {
		t.Fatalf("Rename(NFD path) error: %v", err)
	}
	renamedResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: renamedTorrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			oldTorrentName: newTorrentName,
		},
		Quiet:   true,
		InPlace: true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent(NFC rename) error: %v", err)
	}
	if got, want := renamedResult.ReusedPieces, renamedResult.TotalPieces; got != want {
		t.Errorf("UpdateTorrent(NFC rename).ReusedPieces = %d, want all %d pieces reused", got, want)
	}
	assertUpdateMatchesFullRehash(t, renamedTorrentPath, contentDir, "release", pieceLength, nil)
}

// TestUpdateTorrentNestedRenamePreservesFileKeys verifies mapped entries retain non-structural metadata.
func TestUpdateTorrentNestedRenamePreservesFileKeys(t *testing.T) {
	contentDir := t.TempDir()
	oldPath := filepath.Join(contentDir, "season", "episode.bin")
	newPath := filepath.Join(contentDir, "archive", "episode.bin")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(old path) error: %v", err)
	}
	writeUpdateTestFile(t, oldPath, bytes.Repeat([]byte{'a'}, 65_536))

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
	addUpdateTestFileValues(t, original.MetaInfo, 0, map[string]any{
		"attr":            "p",
		"md5sum":          "keep-md5",
		"sha1":            "keep-sha1",
		"custom-file-key": "keep-custom",
	})
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(new path) error: %v", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("Rename(nested path) error: %v", err)
	}
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			"season/episode.bin": "archive/episode.bin",
		},
		Quiet:   true,
		InPlace: true,
	})
	if err != nil {
		t.Fatalf("UpdateTorrent() error: %v", err)
	}
	if got, want := result.ReusedPieces, result.TotalPieces; got != want {
		t.Errorf("UpdateTorrent().ReusedPieces = %d, want all %d pieces reused", got, want)
	}
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"archive/episode.bin"})

	updated, err := metainfo.LoadFromFile(torrentPath)
	if err != nil {
		t.Fatalf("LoadFromFile(updated) error: %v", err)
	}
	infoMap := make(map[string]any)
	if err := bencode.Unmarshal(updated.InfoBytes, &infoMap); err != nil {
		t.Fatalf("Unmarshal(updated.InfoBytes) error: %v", err)
	}
	files := infoMap["files"].([]any)
	fileMap := files[0].(map[string]any)
	for key, want := range map[string]any{
		"attr":            "p",
		"md5sum":          "keep-md5",
		"sha1":            "keep-sha1",
		"custom-file-key": "keep-custom",
	} {
		if got := fileMap[key]; got != want {
			t.Errorf("UpdateTorrent() file key %q = %v, want %v", key, got, want)
		}
	}
}

// assertUpdateMatchesFullRehash verifies updated metadata and hashes against a clean rebuild.
func assertUpdateMatchesFullRehash(t *testing.T, torrentPath, contentPath, name string, pieceLength uint, wantPaths []string) {
	t.Helper()
	updated, err := metainfo.LoadFromFile(torrentPath)
	if err != nil {
		t.Fatalf("LoadFromFile(updated) error: %v", err)
	}
	updatedInfo, err := updated.UnmarshalInfo()
	if err != nil {
		t.Fatalf("UnmarshalInfo(updated) error: %v", err)
	}
	fullyHashed, err := CreateTorrent(CreateOptions{
		Path:           contentPath,
		Name:           name,
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
	if wantPaths != nil {
		if got := updateTestPaths(updatedInfo.Files); !equalUpdateTestStrings(got, wantPaths) {
			t.Errorf("UpdateTorrent() paths = %v, want %v", got, wantPaths)
		}
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

// readUpdateTestRoot decodes the raw root dictionary without discarding unknown keys.
func readUpdateTestRoot(t *testing.T, torrentPath string) map[string]bencode.Bytes {
	t.Helper()
	data, err := os.ReadFile(torrentPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", torrentPath, err)
	}
	rootMap := make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(data, &rootMap); err != nil {
		t.Fatalf("Unmarshal(root %q) error: %v", torrentPath, err)
	}
	return rootMap
}

func readUpdateTestRawInfo(t *testing.T, torrentPath string) map[string]bencode.Bytes {
	t.Helper()
	rootMap := readUpdateTestRoot(t, torrentPath)
	infoMap := make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(rootMap["info"], &infoMap); err != nil {
		t.Fatalf("Unmarshal(raw info) error: %v", err)
	}
	return infoMap
}

func addUpdateTestRawInfoValues(
	t *testing.T,
	torrentPath string,
	infoValues map[string]bencode.Bytes,
	fileIndex int,
	fileValues map[string]bencode.Bytes,
) {
	t.Helper()
	rootMap := readUpdateTestRoot(t, torrentPath)
	infoMap := make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(rootMap["info"], &infoMap); err != nil {
		t.Fatalf("Unmarshal(raw info) error: %v", err)
	}
	for key, value := range infoValues {
		infoMap[key] = value
	}
	if fileValues != nil {
		var files []map[string]bencode.Bytes
		if err := bencode.Unmarshal(infoMap["files"], &files); err != nil {
			t.Fatalf("Unmarshal(raw files) error: %v", err)
		}
		if fileIndex < 0 || fileIndex >= len(files) {
			t.Fatalf("raw file index %d exceeds %d files", fileIndex, len(files))
		}
		for key, value := range fileValues {
			files[fileIndex][key] = value
		}
		filesBytes, err := bencode.Marshal(files)
		if err != nil {
			t.Fatalf("Marshal(raw files) error: %v", err)
		}
		infoMap["files"] = filesBytes
	}
	infoBytes, err := bencode.Marshal(infoMap)
	if err != nil {
		t.Fatalf("Marshal(raw info) error: %v", err)
	}
	rootMap["info"] = infoBytes
	data, err := bencode.Marshal(rootMap)
	if err != nil {
		t.Fatalf("Marshal(root map) error: %v", err)
	}
	if err := os.WriteFile(torrentPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", torrentPath, err)
	}
}

// addUpdateTestRootValues injects raw root keys into a torrent fixture.
func addUpdateTestRootValues(t *testing.T, torrentPath string, values map[string]any) {
	t.Helper()
	rootMap := readUpdateTestRoot(t, torrentPath)
	for key, value := range values {
		rawValue, err := bencode.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal(root value %q) error: %v", key, err)
		}
		rootMap[key] = rawValue
	}
	data, err := bencode.Marshal(rootMap)
	if err != nil {
		t.Fatalf("Marshal(root map) error: %v", err)
	}
	if err := os.WriteFile(torrentPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", torrentPath, err)
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

// addUpdateTestFileValues injects raw per-file keys into a multi-file torrent fixture.
func addUpdateTestFileValues(t *testing.T, mi *metainfo.MetaInfo, fileIndex int, values map[string]any) {
	t.Helper()
	infoMap := make(map[string]any)
	if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err != nil {
		t.Fatalf("Unmarshal(InfoBytes) error: %v", err)
	}
	files, ok := infoMap["files"].([]any)
	if !ok || fileIndex < 0 || fileIndex >= len(files) {
		t.Fatalf("torrent files metadata does not contain index %d", fileIndex)
	}
	fileMap, ok := files[fileIndex].(map[string]any)
	if !ok {
		t.Fatalf("torrent file %d metadata has unexpected type %T", fileIndex, files[fileIndex])
	}
	for key, value := range values {
		fileMap[key] = value
	}
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
