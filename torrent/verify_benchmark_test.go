package torrent

import (
	"crypto/sha1"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

func BenchmarkPieceVerifierSingleFile(b *testing.B) {
	benchmarkPieceVerifier(b, "single-file", []int64{256 << 20}, 1<<20)
}

func BenchmarkPieceVerifierSeasonPack(b *testing.B) {
	fileSizes := make([]int64, 8)
	for i := range fileSizes {
		fileSizes[i] = 128 << 20
	}
	benchmarkPieceVerifier(b, "season-pack", fileSizes, 1<<20)
}

func benchmarkPieceVerifier(b *testing.B, name string, fileSizes []int64, pieceLen int64) {
	b.Helper()

	tempDir := b.TempDir()
	files, expectedHashes := createTestFilesWithPattern(b, tempDir, fileSizes, pieceLen)

	var totalSize int64
	for _, fileSize := range fileSizes {
		totalSize += fileSize
	}

	pieces := make([]byte, 0, len(expectedHashes)*sha1.Size)
	for _, pieceHash := range expectedHashes {
		pieces = append(pieces, pieceHash...)
	}

	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(totalSize)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			display := NewDisplay(NewFormatter(false))
			display.SetQuiet(true)

			verifier := &pieceVerifier{
				torrentInfo: &metainfo.Info{
					PieceLength: pieceLen,
					Pieces:      pieces,
				},
				display:   display,
				files:     append([]fileEntry(nil), files...),
				pieceLen:  pieceLen,
				numPieces: len(expectedHashes),
			}

			if err := verifier.verifyPieces(0); err != nil {
				b.Fatalf("verifyPieces failed: %v", err)
			}
			if verifier.badPieces != 0 || verifier.goodPieces != uint64(len(expectedHashes)) {
				b.Fatalf("unexpected verification result: good=%d bad=%d", verifier.goodPieces, verifier.badPieces)
			}
		}
	})
}
