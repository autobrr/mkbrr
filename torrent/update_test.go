package torrent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	original.Comment = "keep-comment"
	addUpdateTestInfoValue(t, original.MetaInfo, "custom-key", "keep-value")

	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	require.NoError(t, os.Rename(filepath.Join(contentDir, "a.bin"), filepath.Join(contentDir, "b.bin")))
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
	require.NoError(t, err)
	assert.Equal(t, 2, result.ReusedPieces)
	assert.Equal(t, 1, result.HashedPieces)
	updated, err := metainfo.LoadFromFile(torrentPath)
	require.NoError(t, err)
	updatedInfo, err := updated.UnmarshalInfo()
	require.NoError(t, err)
	fullyHashed, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		Name:           "release",
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	require.NoError(t, err)
	fullyHashedInfo, err := fullyHashed.UnmarshalInfo()
	require.NoError(t, err)
	assert.Equal(t, fullyHashedInfo.Pieces, updatedInfo.Pieces)
	assert.Equal(t, []string{"b.bin", "m.bin", "z.bin"}, updateTestPaths(updatedInfo.Files))
	assert.Equal(t, "keep-comment", updated.Comment)
	infoMap := make(map[string]any)
	require.NoError(t, bencode.Unmarshal(updated.InfoBytes, &infoMap))
	assert.Equal(t, "keep-value", infoMap["custom-key"])
	assert.Equal(t, "source-tag", infoMap["source"])
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	require.NoError(t, os.Remove(filepath.Join(contentDir, " a.bin")))
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.ReusedPieces)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	rootValues := map[string]any{
		"httpseeds":    []string{"https://seed.example/content"},
		"url-list":     []string{"https://webseed.example/content"},
		"x_cross_seed": "keep-me",
	}
	addUpdateTestRootValues(t, torrentPath, rootValues)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	require.NoError(t, err)
	rootMap := readUpdateTestRoot(t, torrentPath)
	for key, value := range rootValues {
		want, err := bencode.Marshal(value)
		require.NoError(t, err)
		assert.Equal(t, bencode.Bytes(want), rootMap[key])
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	bigInteger := bencode.Bytes("i9223372036854775808e")
	addUpdateTestRawInfoValues(t, torrentPath,
		map[string]bencode.Bytes{"x-bigint": bigInteger},
		0,
		map[string]bencode.Bytes{"x-file-bigint": bigInteger},
	)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	require.NoError(t, err)
	infoMap := readUpdateTestRawInfo(t, torrentPath)
	assert.Equal(t, bigInteger, infoMap["x-bigint"])
	var files []map[string]bencode.Bytes
	require.NoError(t, bencode.Unmarshal(infoMap["files"], &files))
	assert.Equal(t, bigInteger, files[0]["x-file-bigint"])
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
				require.NoError(t, err)
				torrentPath := filepath.Join(t.TempDir(), "input.torrent")
				writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

				_, err = UpdateTorrent(UpdateOptions{
					TorrentPath: torrentPath,
					ContentPath: contentPath,
					OutputPath:  test.setup(t, contentPath),
					Quiet:       true,
				})
				require.ErrorContains(t, err, "must not replace the content file")
				got, err := os.ReadFile(contentPath)
				require.NoError(t, err)
				assert.Equal(t, contentBytes, got)
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
		require.NoError(t, err)
		torrentPath := filepath.Join(t.TempDir(), "input.torrent")
		writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
		outputPath := filepath.Join(contentDir, "nested", "output.data")

		_, err = UpdateTorrent(UpdateOptions{
			TorrentPath: torrentPath,
			ContentPath: contentDir,
			OutputPath:  outputPath,
			Quiet:       true,
		})
		require.ErrorContains(t, err, "must not be inside the content directory")
		_, err = os.Stat(outputPath)
		require.True(t, os.IsNotExist(err))
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "input.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	outputPath := filepath.Join(t.TempDir(), "output.torrent")
	if err := os.Symlink(contentPath, outputPath); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		OutputPath:  outputPath,
		Quiet:       true,
	})
	require.NoError(t, err)
	gotContent, err := os.ReadFile(contentPath)
	require.NoError(t, err)
	assert.Equal(t, contentBytes, gotContent)
	outputInfo, err := os.Lstat(outputPath)
	require.NoError(t, err)
	assert.Zero(t, outputInfo.Mode()&os.ModeSymlink)
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: outputPath,
		ContentPath: contentPath,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(100), verification.Completion)
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
			require.NoError(t, os.Mkdir(contentDir, 0o755))
			contentAlias := filepath.Join(parent, test.aliasName)
			contentInfo, err := os.Stat(contentDir)
			require.NoError(t, err)
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
			require.NoError(t, err)
			torrentPath := filepath.Join(t.TempDir(), "input.torrent")
			writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
			outputPath := filepath.Join(contentAlias, "output.torrent")

			_, err = UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: contentDir,
				OutputPath:  outputPath,
				Quiet:       true,
			})
			require.ErrorContains(t, err, "inside the content directory")
			_, err = os.Stat(outputPath)
			require.True(t, os.IsNotExist(err))
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
	require.NoError(t, err)
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
			require.NoError(t, os.Remove(outputParent))
			require.NoError(t, os.Symlink(contentDir, outputParent))
		},
	})
	require.ErrorContains(t, err, "output parent changed during update")
	gotContent, err := os.ReadFile(contentPath)
	require.NoError(t, err)
	assert.Equal(t, contentBytes, gotContent)
	_, err = os.Stat(filepath.Join(safeOutputDir, "victim.bin"))
	require.True(t, os.IsNotExist(err))
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
	require.NoError(t, err)
	addUpdateTestFileValues(t, original.MetaInfo, 0, map[string]any{"length": int64(-1)})
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	before, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	require.ErrorContains(t, err, "negative length")
	after, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	addUpdateTestRawInfoValues(t, torrentPath, map[string]bencode.Bytes{
		"length": bencode.Bytes("i0e"),
	}, 0, nil)
	before, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	require.ErrorContains(t, err, "both files and length")
	after, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "malformed.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	addUpdateTestRawInfoValues(t, torrentPath, map[string]bencode.Bytes{
		"name": bencode.Bytes("0:"),
	}, 0, nil)
	before, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	require.ErrorContains(t, err, "invalid name")
	after, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "extreme.torrent")
	writeUpdateTestTorrent(t, torrentPath, &metainfo.MetaInfo{InfoBytes: infoBytes})

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		InPlace:     true,
		Force:       true,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.False(t, result.TotalPieces != 1 || result.HashedPieces != 1)
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(100), verification.Completion)
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
	require.NoError(t, err)
	originalInfo, err := original.UnmarshalInfo()
	require.NoError(t, err)
	wantPaths := []string{"link1.bin", "link2.bin", "real.bin"}
	require.Equal(t, wantPaths, updateTestPaths(originalInfo.Files))
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		InPlace:     true,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, result.TotalPieces, result.ReusedPieces)
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, wantPaths)
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(100), verification.Completion)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentAlias,
		InPlace:     true,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, result.TotalPieces, result.ReusedPieces)
	verification, err := VerifyData(VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentAlias,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, float64(100), verification.Completion)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	originalBytes, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.Dir(torrentPath), "release.updated.torrent"), result.OutputPath)
	afterBytes, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	assert.Equal(t, originalBytes, afterBytes)
	assertUpdateMatchesFullRehash(t, result.OutputPath, contentDir, "release", pieceLength, []string{"a.bin"})
	outputBytes, err := os.ReadFile(result.OutputPath)
	require.NoError(t, err)
	_, err = UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
	})
	require.ErrorContains(t, err, "default output")
	afterRefusal, err := os.ReadFile(result.OutputPath)
	require.NoError(t, err)
	assert.Equal(t, outputBytes, afterRefusal)
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
			require.NoError(t, err)
			torrentPath := filepath.Join(t.TempDir(), "release.torrent")
			writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
			originalBytes, err := os.ReadFile(torrentPath)
			require.NoError(t, err)
			_, err = UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: unrelatedDir,
				InPlace:     true,
				Quiet:       true,
			})
			wantError := fmt.Sprintf("0 reusable pieces (existing torrent: 3, updated torrent: %d)", test.updatedPieces)
			require.ErrorContains(t, err, wantError)
			afterRefusal, err := os.ReadFile(torrentPath)
			require.NoError(t, err)
			assert.Equal(t, originalBytes, afterRefusal)
			result, err := UpdateTorrent(UpdateOptions{
				TorrentPath: torrentPath,
				ContentPath: unrelatedDir,
				InPlace:     true,
				Force:       true,
				Quiet:       true,
			})
			require.NoError(t, err)
			assert.Equal(t, 0, result.ReusedPieces)
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
	require.NoError(t, err)
	fallbackTorrentPath := filepath.Join(t.TempDir(), "fallback.torrent")
	explicitTorrentPath := filepath.Join(t.TempDir(), "explicit.torrent")
	writeUpdateTestTorrent(t, fallbackTorrentPath, original.MetaInfo)
	writeUpdateTestTorrent(t, explicitTorrentPath, original.MetaInfo)
	require.NoError(t, os.Rename(filepath.Join(contentDir, "a.bin"), filepath.Join(contentDir, "c.bin")))
	require.NoError(t, os.Rename(filepath.Join(contentDir, "b.bin"), filepath.Join(contentDir, "d.bin")))
	fallbackResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: fallbackTorrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
		Force:       true,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, fallbackResult.ReusedPieces)
	assert.Equal(t, 2, fallbackResult.HashedPieces)
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
	require.NoError(t, err)
	assert.Equal(t, 2, explicitResult.ReusedPieces)
	assert.Equal(t, 0, explicitResult.HashedPieces)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	require.NoError(t, os.Remove(filepath.Join(contentDir, "old.bin")))
	writeUpdateTestFile(t, filepath.Join(contentDir, "new.bin"), bytes.Repeat([]byte{'b'}, 65_536))

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ReusedPieces)
	assert.Equal(t, 1, result.HashedPieces)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)

	for _, filePath := range removedPaths {
		require.NoError(t, os.Remove(filepath.Join(contentDir, filePath)))
	}

	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, result.TotalPieces, result.ReusedPieces)
	assert.Equal(t, 0, result.HashedPieces)
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
	require.NoError(t, err)
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	require.NoError(t, os.Remove(filepath.Join(contentDir, "b.bin")))
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Quiet:       true,
		InPlace:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.ReusedPieces)
	assert.Equal(t, 1, result.HashedPieces)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	assert.Equal(t, 0, result.ReusedPieces)
	assert.Equal(t, result.TotalPieces, result.HashedPieces)
	assertUpdateMatchesFullRehash(t, outputPath, replacementPath, "original.bin", pieceLength, nil)

	t.Run("sets default POSIX permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX file modes are not meaningful on Windows")
		}

		outputInfo, err := os.Stat(outputPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), outputInfo.Mode().Perm())
	})
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	assert.Equal(t, unchangedResult.TotalPieces, unchangedResult.ReusedPieces)
	assertUpdateMatchesFullRehash(t, unchangedTorrentPath, contentDir, "release", pieceLength, nil)

	newDiskName := "renome\u0301.bin"
	newTorrentName := "renom\u00e9.bin"
	require.NoError(t, os.Rename(filepath.Join(contentDir, oldDiskName), filepath.Join(contentDir, newDiskName)))
	renamedResult, err := UpdateTorrent(UpdateOptions{
		TorrentPath: renamedTorrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			oldTorrentName: newTorrentName,
		},
		Quiet:   true,
		InPlace: true,
	})
	require.NoError(t, err)
	assert.Equal(t, renamedResult.TotalPieces, renamedResult.ReusedPieces)
	assertUpdateMatchesFullRehash(t, renamedTorrentPath, contentDir, "release", pieceLength, nil)
}

// TestUpdateTorrentNestedRenamePreservesFileKeys verifies mapped entries retain non-structural metadata.
func TestUpdateTorrentNestedRenamePreservesFileKeys(t *testing.T) {
	contentDir := t.TempDir()
	oldPath := filepath.Join(contentDir, "season", "episode.bin")
	newPath := filepath.Join(contentDir, "archive", "episode.bin")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0o755))
	writeUpdateTestFile(t, oldPath, bytes.Repeat([]byte{'a'}, 65_536))

	pieceLength := uint(16)
	original, err := CreateTorrent(CreateOptions{
		Path:           contentDir,
		Name:           "release",
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	require.NoError(t, err)
	addUpdateTestFileValues(t, original.MetaInfo, 0, map[string]any{
		"attr":            "p",
		"md5sum":          "keep-md5",
		"sha1":            "keep-sha1",
		"custom-file-key": "keep-custom",
	})
	torrentPath := filepath.Join(t.TempDir(), "release.torrent")
	writeUpdateTestTorrent(t, torrentPath, original.MetaInfo)
	require.NoError(t, os.MkdirAll(filepath.Dir(newPath), 0o755))
	require.NoError(t, os.Rename(oldPath, newPath))
	result, err := UpdateTorrent(UpdateOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Renames: map[string]string{
			"season/episode.bin": "archive/episode.bin",
		},
		Quiet:   true,
		InPlace: true,
	})
	require.NoError(t, err)
	assert.Equal(t, result.TotalPieces, result.ReusedPieces)
	assertUpdateMatchesFullRehash(t, torrentPath, contentDir, "release", pieceLength, []string{"archive/episode.bin"})

	updated, err := metainfo.LoadFromFile(torrentPath)
	require.NoError(t, err)
	infoMap := make(map[string]any)
	require.NoError(t, bencode.Unmarshal(updated.InfoBytes, &infoMap))
	files := infoMap["files"].([]any)
	fileMap := files[0].(map[string]any)
	for key, want := range map[string]any{
		"attr":            "p",
		"md5sum":          "keep-md5",
		"sha1":            "keep-sha1",
		"custom-file-key": "keep-custom",
	} {
		assert.Equal(t, want, fileMap[key])
	}
}

// assertUpdateMatchesFullRehash verifies updated metadata and hashes against a clean rebuild.
func assertUpdateMatchesFullRehash(t *testing.T, torrentPath, contentPath, name string, pieceLength uint, wantPaths []string) {
	t.Helper()
	updated, err := metainfo.LoadFromFile(torrentPath)
	require.NoError(t, err)
	updatedInfo, err := updated.UnmarshalInfo()
	require.NoError(t, err)
	fullyHashed, err := CreateTorrent(CreateOptions{
		Path:           contentPath,
		Name:           name,
		PieceLengthExp: &pieceLength,
		Quiet:          true,
	})
	require.NoError(t, err)
	fullyHashedInfo, err := fullyHashed.UnmarshalInfo()
	require.NoError(t, err)
	assert.Equal(t, fullyHashedInfo.Pieces, updatedInfo.Pieces)
	if wantPaths != nil {
		assert.Equal(t, wantPaths, updateTestPaths(updatedInfo.Files))
	}
}

// writeUpdateTestFile creates content fixtures with deterministic bytes.
func writeUpdateTestFile(t *testing.T, filePath string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filePath, content, 0o644))
}

// writeUpdateTestTorrent serializes metainfo for update tests.
func writeUpdateTestTorrent(t *testing.T, torrentPath string, mi *metainfo.MetaInfo) {
	t.Helper()
	file, err := os.Create(torrentPath)
	require.NoError(t, err)
	err = mi.Write(file)
	if err != nil {
		_ = file.Close()
	}
	require.NoError(t, err)
	require.NoError(t, file.Close())
}

// readUpdateTestRoot decodes the raw root dictionary without discarding unknown keys.
func readUpdateTestRoot(t *testing.T, torrentPath string) map[string]bencode.Bytes {
	t.Helper()
	data, err := os.ReadFile(torrentPath)
	require.NoError(t, err)
	rootMap := make(map[string]bencode.Bytes)
	require.NoError(t, bencode.Unmarshal(data, &rootMap))
	return rootMap
}

func readUpdateTestRawInfo(t *testing.T, torrentPath string) map[string]bencode.Bytes {
	t.Helper()
	rootMap := readUpdateTestRoot(t, torrentPath)
	infoMap := make(map[string]bencode.Bytes)
	require.NoError(t, bencode.Unmarshal(rootMap["info"], &infoMap))
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
	require.NoError(t, bencode.Unmarshal(rootMap["info"], &infoMap))
	for key, value := range infoValues {
		infoMap[key] = value
	}
	if fileValues != nil {
		var files []map[string]bencode.Bytes
		require.NoError(t, bencode.Unmarshal(infoMap["files"], &files))
		require.False(t, fileIndex < 0 || fileIndex >= len(files))
		for key, value := range fileValues {
			files[fileIndex][key] = value
		}
		filesBytes, err := bencode.Marshal(files)
		require.NoError(t, err)
		infoMap["files"] = filesBytes
	}
	infoBytes, err := bencode.Marshal(infoMap)
	require.NoError(t, err)
	rootMap["info"] = infoBytes
	data, err := bencode.Marshal(rootMap)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(torrentPath, data, 0o644))
}

// addUpdateTestRootValues injects raw root keys into a torrent fixture.
func addUpdateTestRootValues(t *testing.T, torrentPath string, values map[string]any) {
	t.Helper()
	rootMap := readUpdateTestRoot(t, torrentPath)
	for key, value := range values {
		rawValue, err := bencode.Marshal(value)
		require.NoError(t, err)
		rootMap[key] = rawValue
	}
	data, err := bencode.Marshal(rootMap)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(torrentPath, data, 0o644))
}

// addUpdateTestInfoValue injects a custom info key to verify lossless metadata preservation.
func addUpdateTestInfoValue(t *testing.T, mi *metainfo.MetaInfo, key string, value any) {
	t.Helper()
	infoMap := make(map[string]any)
	require.NoError(t, bencode.Unmarshal(mi.InfoBytes, &infoMap))
	infoMap[key] = value
	infoBytes, err := bencode.Marshal(infoMap)
	require.NoError(t, err)
	mi.InfoBytes = infoBytes
}

// addUpdateTestFileValues injects raw per-file keys into a multi-file torrent fixture.
func addUpdateTestFileValues(t *testing.T, mi *metainfo.MetaInfo, fileIndex int, values map[string]any) {
	t.Helper()
	infoMap := make(map[string]any)
	require.NoError(t, bencode.Unmarshal(mi.InfoBytes, &infoMap))
	files, ok := infoMap["files"].([]any)
	require.False(t, !ok || fileIndex < 0 || fileIndex >= len(files))
	fileMap, ok := files[fileIndex].(map[string]any)
	require.True(t, ok)
	for key, value := range values {
		fileMap[key] = value
	}
	infoBytes, err := bencode.Marshal(infoMap)
	require.NoError(t, err)
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
