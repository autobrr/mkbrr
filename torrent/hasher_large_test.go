//go:build large_tests
// +build large_tests

package torrent

import (
	"fmt"
	"runtime"
	"testing"
)

// TestPieceHasher_LargeFiles tests the hasher with large file scenarios.
// These tests are skipped by default and in CI due to their resource requirements.
// Run with: go test -v -tags=large_tests
func TestPieceHasher_LargeFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping large file tests on Windows")
	}

	if testing.Short() {
		t.Skip("skipping large file tests")
	}

	tests := []struct {
		name      string
		numFiles  int
		fileSize  int64
		pieceLen  int64
		numPieces int
	}{
		{
			name:      "4k movie remux",
			numFiles:  1,
			fileSize:  64 << 30, // 64GB
			pieceLen:  1 << 24,  // 16MB - PTP recommendation for large remuxes
			numPieces: 4096,
		},
		{
			name:      "1080p season pack",
			numFiles:  10,      // 10 episodes
			fileSize:  4 << 30, // 4GB per episode (~40GB total)
			pieceLen:  1 << 23, // 8MB - better for multi-file packs
			numPieces: 5120,
		},
		{
			name:      "game distribution",
			numFiles:  100,     // Many small files
			fileSize:  1 << 28, // 256MB per file (~25GB total)
			pieceLen:  1 << 21, // 2MB - better for smaller individual files
			numPieces: 12800,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, expectedHashes := createTestFilesFast(t, tt.numFiles, tt.fileSize, tt.pieceLen)
			hasher := NewPieceHasher(files, tt.pieceLen, tt.numPieces, &mockDisplay{})

			// test with different worker counts
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
