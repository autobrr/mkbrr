package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// TestPieceHasher_Concurrent tests the hasher with various real-world scenarios.
// Test cases are designed to cover common torrent types and sizes:
//   - Piece sizes: 64KB for small files (standard minimum)
//     4MB for large files
//   - Worker counts: 1 (sequential baseline)
//     2 (minimal concurrency)
//     4 (typical desktop CPU)
//     8 (high-end desktop/server)
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
			name:      "1080p episode",
			numFiles:  1,
			fileSize:  4 << 30, // 4GB
			pieceLen:  1 << 22, // 4MB
			numPieces: 1024,
		},
		{
			name:      "multi-file album",
			numFiles:  12,       // typical album length
			fileSize:  40 << 20, // 40MB per track
			pieceLen:  1 << 16,  // 64KB pieces
			numPieces: 7680,     // ~480MB total
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip large tests in short mode
			totalSize := tt.fileSize * int64(tt.numFiles)
			if testing.Short() && totalSize > 1<<30 { // Skip if total size > 1GB in short mode
				t.Skipf("skipping large file test %s in short mode", tt.name)
			}

			files, expectedHashes := createTestFilesFast(t, tt.numFiles, tt.fileSize, tt.pieceLen)
			hasher := NewPieceHasher(files, tt.pieceLen, tt.numPieces, &mockDisplay{})

			// Test with different worker counts
			workerCounts := []int{1, 2, 4, 8}
			for _, workers := range workerCounts {
				t.Run(fmt.Sprintf("workers_%d", workers), func(t *testing.T) {
					if err := hasher.hashPieces(workers); err != nil {
						t.Fatalf("hashPieces failed with %d workers: %v", workers, err)
					}
					verifyHashes(t, hasher.pieces, expectedHashes)
				})
			}
		})
	}
}

// createTestFilesFast creates test files more efficiently using sparse files
func createTestFilesFast(t *testing.T, numFiles int, fileSize, pieceLen int64) ([]fileEntry, [][]byte) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "hasher_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	var files []fileEntry
	var expectedHashes [][]byte
	var offset int64

	// Create a repeatable pattern that's more representative of real data
	pattern := make([]byte, pieceLen)
	for i := range pattern {
		// Create a pseudo-random but deterministic pattern
		pattern[i] = byte((i*7 + 13) % 251) // Prime numbers help create distribution
	}

	for i := 0; i < numFiles; i++ {
		path := filepath.Join(tempDir, fmt.Sprintf("test_file_%d", i))

		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		// Write pattern at start and end of file to simulate real data
		// while keeping the file sparse in the middle
		if _, err := f.Write(pattern); err != nil {
			f.Close()
			t.Fatalf("failed to write pattern: %v", err)
		}

		if _, err := f.Seek(fileSize-int64(len(pattern)), io.SeekStart); err != nil {
			f.Close()
			t.Fatalf("failed to seek: %v", err)
		}

		if _, err := f.Write(pattern); err != nil {
			f.Close()
			t.Fatalf("failed to write pattern: %v", err)
		}

		if err := f.Truncate(fileSize); err != nil {
			f.Close()
			t.Fatalf("failed to truncate file: %v", err)
		}
		f.Close()

		files = append(files, fileEntry{
			path:   path,
			length: fileSize,
			offset: offset,
		})
		offset += fileSize

		// Calculate expected hashes with the pattern
		h := sha1.New()
		for pos := int64(0); pos < fileSize; pos += pieceLen {
			h.Reset()
			if pos == 0 || pos >= fileSize-pieceLen {
				h.Write(pattern) // Use pattern for first and last pieces
			} else {
				h.Write(make([]byte, pieceLen)) // Zero bytes for middle pieces
			}
			expectedHashes = append(expectedHashes, h.Sum(nil))
		}
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
		skipOnWin bool
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
			skipOnWin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnWin && runtime.GOOS == "windows" {
				t.Skip("skipping unreadable file test on Windows")
			}
			files := tt.setup()
			hasher := NewPieceHasher(files, tt.pieceLen, tt.numPieces, &mockDisplay{})

			err := hasher.hashPieces(2)
			if (err != nil) != tt.wantErr {
				t.Errorf("hashPieces() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPieceHasher_RaceConditions tests concurrent access using a FLAC album scenario.
// FLAC albums are ideal for race testing because:
// - Multiple small-to-medium files (12 tracks, 40MB each)
// - Small piece size (64KB) creates more concurrent operations
func TestPieceHasher_RaceConditions(t *testing.T) {
	// Use the multi-file album test case from TestPieceHasher_Concurrent
	// but run multiple hashers concurrently to stress test race conditions
	numFiles := 12
	fileSize := int64(40 << 20) // 40MB per track
	pieceLen := int64(1 << 16)  // 64KB pieces
	numPieces := 7680           // ~480MB total

	tempDir, err := os.MkdirTemp("", "hasher_race_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	files, expectedHashes := createTestFilesFast(t, numFiles, fileSize, pieceLen)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasher := NewPieceHasher(files, pieceLen, numPieces, &mockDisplay{})
			if err := hasher.hashPieces(4); err != nil {
				t.Errorf("hashPieces failed: %v", err)
				return
			}
			verifyHashes(t, hasher.pieces, expectedHashes)
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

	files, expectedHashes := createTestFilesFast(t, 1, 1<<16, 1<<16) // 1 file, 64KB

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
