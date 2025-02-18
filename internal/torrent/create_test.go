package torrent

import (
	"testing"
)

func Test_calculatePieceLength(t *testing.T) {
	tests := []struct {
		name           string
		totalSize      int64
		maxPieceLength *uint
		piecesTarget   *uint
		want           uint
		wantPieces     *uint // expected number of pieces (approximate)
	}{
		{
			name:      "small file should use minimum piece length",
			totalSize: 1 << 10, // 1 KiB
			want:      14,      // 16 KiB pieces
		},
		{
			name:      "1GB file should use 1MB pieces",
			totalSize: 1 << 30, // 1 GiB
			want:      20,      // 1 MiB pieces
		},
		{
			name:      "large file should use large pieces",
			totalSize: 1 << 40, // 1 TiB
			want:      24,      // 16 MiB pieces
		},
		{
			name:      "zero size should use minimum piece length",
			totalSize: 0,
			want:      14, // 16 KiB pieces
		},
		{
			name:           "max piece length should be respected",
			totalSize:      1 << 40, // 1 TiB
			maxPieceLength: uint_ptr(22),
			want:           22, // 4 MiB pieces
		},
		{
			name:           "max piece length below minimum should use minimum",
			totalSize:      1 << 40,
			maxPieceLength: uint_ptr(10),
			want:           14, // 16 KiB pieces
		},
		{
			name:           "max piece length above maximum should use maximum",
			totalSize:      1 << 40,
			maxPieceLength: uint_ptr(30),
			want:           24, // 16 MiB pieces
		},
		{
			name:         "target 1000 pieces for 1GB file",
			totalSize:    1 << 30, // 1 GiB
			piecesTarget: uint_ptr(1000),
			want:         21, // 2 MiB pieces, ~512 pieces
			wantPieces:   uint_ptr(512),
		},
		{
			name:         "target 1000 pieces for 10GB file",
			totalSize:    10 << 30, // 10 GiB
			piecesTarget: uint_ptr(1000),
			want:         24, // 16 MiB pieces, ~640 pieces
			wantPieces:   uint_ptr(640),
		},
		{
			name:           "target pieces should respect max piece length",
			totalSize:      10 << 30, // 10 GiB
			piecesTarget:   uint_ptr(100),
			maxPieceLength: uint_ptr(22),
			want:           22, // 4 MiB pieces, ~2560 pieces
			wantPieces:     uint_ptr(2560),
		},
		{
			name:         "target pieces should respect minimum piece length",
			totalSize:    1 << 20, // 1 MiB
			piecesTarget: uint_ptr(1000),
			want:         14, // 16 KiB pieces, ~64 pieces
			wantPieces:   uint_ptr(64),
		},
		{
			name:         "zero target pieces should use default calculation",
			totalSize:    1 << 30, // 1 GiB
			piecesTarget: uint_ptr(0),
			want:         20, // 1 MiB pieces
		},
		{
			name:       "78GiB file should use maximum piece length",
			totalSize:  78 << 30, // 78 GiB
			want:       24,       // 16 MiB pieces
			wantPieces: uint_ptr(5230),
		},
		{
			name:         "78GiB file with target 1000 pieces",
			totalSize:    78 << 30, // 78 GiB
			piecesTarget: uint_ptr(1000),
			want:         24, // limited to 16 MiB pieces
			wantPieces:   uint_ptr(5230),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePieceLength(tt.totalSize, tt.maxPieceLength, tt.piecesTarget)
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

// uint_ptr returns a pointer to the given uint
func uint_ptr(v uint) *uint {
	return &v
}
