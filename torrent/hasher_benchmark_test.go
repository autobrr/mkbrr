package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkPieceHasherSingleFile(b *testing.B) {
	benchmarkPieceHasher(b, "single-file", 1, 256<<20, 1<<20)
}

func BenchmarkPieceHasherSeasonPack(b *testing.B) {
	benchmarkPieceHasher(b, "season-pack", 8, 128<<20, 1<<20)
}

func benchmarkPieceHasher(b *testing.B, name string, numFiles int, fileSize, pieceLen int64) {
	b.Helper()

	files := createBenchmarkFiles(b, numFiles, fileSize, pieceLen)
	totalSize := int64(numFiles) * fileSize
	numPieces := int((totalSize + pieceLen - 1) / pieceLen)

	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(totalSize)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			hasher := NewPieceHasher(files, pieceLen, numPieces, &mockDisplay{}, false)
			if err := hasher.hashPieces(0); err != nil {
				b.Fatalf("hashPieces failed: %v", err)
			}
		}
	})
}

func createBenchmarkFiles(b *testing.B, numFiles int, fileSize, pieceLen int64) []fileEntry {
	b.Helper()

	tempDir := b.TempDir()
	pattern := make([]byte, pieceLen)
	for i := range pattern {
		pattern[i] = byte((i*7 + 13) % 251)
	}

	files := make([]fileEntry, 0, numFiles)
	var offset int64
	for i := 0; i < numFiles; i++ {
		path := filepath.Join(tempDir, fmt.Sprintf("bench_file_%02d.bin", i))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			b.Fatalf("failed to create benchmark file: %v", err)
		}

		written := int64(0)
		for written < fileSize {
			chunk := min(pieceLen, fileSize-written)
			if _, err := f.Write(pattern[:chunk]); err != nil {
				_ = f.Close()
				b.Fatalf("failed to write benchmark file: %v", err)
			}
			written += chunk
		}
		if err := f.Close(); err != nil {
			b.Fatalf("failed to close benchmark file: %v", err)
		}

		files = append(files, fileEntry{
			path:   path,
			length: fileSize,
			offset: offset,
		})
		offset += fileSize
	}

	return files
}
