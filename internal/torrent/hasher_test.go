package torrent

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/schollz/progressbar/v3"
)

// mockDisplay implements Display interface for testing
type mockDisplay struct{}

func (m *mockDisplay) ShowProgress(total int) *progressbar.ProgressBar { return nil }
func (m *mockDisplay) UpdateProgress(count int)                        {}
func (m *mockDisplay) FinishProgress()                                 {}
func (m *mockDisplay) ShowMessage(message string)                      {}
func (m *mockDisplay) ShowWarning(message string)                      {}
func (m *mockDisplay) ShowError(message string)                        {}

func TestPieceHasher_Concurrent(t *testing.T) {
	tests := []struct {
		name      string
		numFiles  int
		fileSize  int64
		pieceLen  int64
		numPieces int
	}{
		{
			name:      "single small file",
			numFiles:  1,
			fileSize:  1 << 20, // 1MB
			pieceLen:  1 << 16, // 64KB
			numPieces: 16,
		},
		{
			name:      "multiple small files",
			numFiles:  5,
			fileSize:  1 << 19, // 512KB each
			pieceLen:  1 << 16, // 64KB
			numPieces: 40,
		},
		{
			name:      "large file spanning multiple pieces",
			numFiles:  1,
			fileSize:  1 << 24, // 16MB
			pieceLen:  1 << 20, // 1MB
			numPieces: 16,
		},
		{
			name:      "file size not divisible by piece length",
			numFiles:  1,
			fileSize:  (1 << 16) + 100, // 64KB + 100 bytes
			pieceLen:  1 << 16,         // 64KB
			numPieces: 2,
		},
		{
			name:      "single extremely large file",
			numFiles:  1,
			fileSize:  1 << 30, // 1GB
			pieceLen:  1 << 20, // 1MB
			numPieces: 1024,
		},
		{
			name:      "maximum workers",
			numFiles:  2,
			fileSize:  1 << 20, // 1MB each
			pieceLen:  1 << 16, // 64KB
			numPieces: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create temporary directory for test files
			tempDir, err := os.MkdirTemp("", "hasher_test")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// create test files and calculate expected hashes
			files, expectedHashes := createTestFiles(t, tempDir, tt.numFiles, tt.fileSize, tt.pieceLen)

			// create hasher
			hasher := NewPieceHasher(files, tt.pieceLen, tt.numPieces, &mockDisplay{})

			// run concurrent hashing multiple times to increase chance of catching races
			for i := 0; i < 3; i++ {
				if err := hasher.hashPieces(4); err != nil {
					t.Fatalf("hashPieces failed: %v", err)
				}

				// verify hashes
				verifyHashes(t, hasher.pieces, expectedHashes)

				// verify no piece was missed
				for i, hash := range hasher.pieces {
					if len(hash) != sha1.Size {
						t.Errorf("piece %d: invalid hash length: got %d, want %d", i, len(hash), sha1.Size)
					}
				}
			}
		})
	}
}

func createTestFiles(t *testing.T, dir string, numFiles int, fileSize, pieceLen int64) ([]fileEntry, [][]byte) {
	t.Helper()

	var files []fileEntry
	var offset int64
	var allData []byte

	// create files with deterministic content
	for i := 0; i < numFiles; i++ {
		path := filepath.Join(dir, filepath.FromSlash(filepath.Join("test", "file"+string(rune('a'+i)))))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// generate deterministic content
		data := make([]byte, fileSize)
		for j := range data {
			data[j] = byte((j + i) % 256)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		files = append(files, fileEntry{
			path:   path,
			length: fileSize,
			offset: offset,
		})

		allData = append(allData, data...)
		offset += fileSize
	}

	// calculate expected piece hashes
	var expectedHashes [][]byte
	totalSize := int64(len(allData))
	for offset := int64(0); offset < totalSize; offset += pieceLen {
		end := offset + pieceLen
		if end > totalSize {
			end = totalSize
		}

		h := sha1.New()
		h.Write(allData[offset:end])
		expectedHashes = append(expectedHashes, h.Sum(nil))
	}

	return files, expectedHashes
}

func verifyHashes(t *testing.T, got, want [][]byte) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("piece count mismatch: got %d, want %d", len(got), len(want))
	}

	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("piece %d hash mismatch:\ngot  %x\nwant %x", i, got[i], want[i])
		}
	}
}

// TestPieceHasher_EdgeCases tests various edge cases and error conditions
func TestPieceHasher_EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hasher_test_edge")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		setup     func() []fileEntry
		pieceLen  int64
		numPieces int
		wantErr   bool
	}{
		{
			name: "non-existent file",
			setup: func() []fileEntry {
				return []fileEntry{{
					path:   filepath.Join(tempDir, "nonexistent"),
					length: 1024,
					offset: 0,
				}}
			},
			pieceLen:  64,
			numPieces: 16,
			wantErr:   true,
		},
		{
			name: "empty file",
			setup: func() []fileEntry {
				path := filepath.Join(tempDir, "empty")
				if err := os.WriteFile(path, []byte{}, 0644); err != nil {
					t.Fatalf("failed to create empty file: %v", err)
				}
				return []fileEntry{{
					path:   path,
					length: 0,
					offset: 0,
				}}
			},
			pieceLen:  64,
			numPieces: 1,
			wantErr:   false,
		},
		{
			name: "unreadable file",
			setup: func() []fileEntry {
				path := filepath.Join(tempDir, "unreadable")
				if err := os.WriteFile(path, []byte("test"), 0000); err != nil {
					t.Fatalf("failed to create unreadable file: %v", err)
				}
				return []fileEntry{{
					path:   path,
					length: 4,
					offset: 0,
				}}
			},
			pieceLen:  64,
			numPieces: 1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.setup()
			hasher := NewPieceHasher(files, tt.pieceLen, tt.numPieces, &mockDisplay{})

			err := hasher.hashPieces(2)
			if (err != nil) != tt.wantErr {
				t.Errorf("hashPieces() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPieceHasher_RaceConditions(t *testing.T) {
	// Run with -race flag to detect races
	tempDir, err := os.MkdirTemp("", "hasher_race_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	files, expectedHashes := createTestFiles(t, tempDir, 3, 1<<20, 1<<16) // 3 files, 1MB each, 64KB pieces

	// Run multiple hashers concurrently on the same files
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasher := NewPieceHasher(files, 1<<16, len(expectedHashes), &mockDisplay{})
			if err := hasher.hashPieces(4); err != nil {
				t.Errorf("hashPieces failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestPieceHasher_NoFiles(t *testing.T) {
	hasher := NewPieceHasher([]fileEntry{}, 1<<16, 0, &mockDisplay{})

	err := hasher.hashPieces(0)
	if err != nil {
		t.Errorf("hashPieces() with no files should not return an error, got %v", err)
	}

	if len(hasher.pieces) != 0 {
		t.Errorf("expected 0 pieces, got %d", len(hasher.pieces))
	}
}

func TestPieceHasher_ZeroWorkers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hasher_zero_workers")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files := []fileEntry{{
		path:   filepath.Join(tempDir, "test"),
		length: 1 << 16,
		offset: 0,
	}}
	hasher := NewPieceHasher(files, 1<<16, 1, &mockDisplay{})

	err = hasher.hashPieces(0)
	if err == nil {
		t.Errorf("expected error when using zero workers, got nil")
	} else {
		expectedErrMsg := "number of workers must be greater than zero when files are present"
		if err.Error() != expectedErrMsg {
			t.Errorf("unexpected error message: got '%v', want '%v'", err.Error(), expectedErrMsg)
		}
	}
}

func TestPieceHasher_CorruptedData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hasher_corrupt_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files, expectedHashes := createTestFiles(t, tempDir, 1, 1<<16, 1<<16) // 1 file, 64KB

	// Corrupt the file by modifying a byte
	corruptedPath := files[0].path
	data, err := os.ReadFile(corruptedPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	data[0] ^= 0xFF // Flip bits of first byte
	if err := os.WriteFile(corruptedPath, data, 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	hasher := NewPieceHasher(files, 1<<16, 1, &mockDisplay{})
	if err := hasher.hashPieces(1); err != nil {
		t.Fatalf("hashPieces failed: %v", err)
	}

	if bytes.Equal(hasher.pieces[0], expectedHashes[0]) {
		t.Errorf("expected hash mismatch due to corrupted data, but hashes matched")
	}
}
