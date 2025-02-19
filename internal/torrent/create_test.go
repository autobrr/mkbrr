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
		trackerURL     string
		want           uint
		wantPieces     *uint // expected number of pieces (approximate)
	}{
		{
			name:      "small file should use minimum piece length",
			totalSize: 1 << 10, // 1 KiB
			want:      16,      // 64 KiB pieces
		},
		{
			name:      "1GB file should use 2MB pieces",
			totalSize: 1 << 30, // 1 GiB
			want:      21,      // 2 MiB pieces
		},
		{
			name:      "large file should use large pieces",
			totalSize: 1 << 40, // 1 TiB
			want:      24,      // 16 MiB pieces
		},
		{
			name:      "zero size should use minimum piece length",
			totalSize: 0,
			want:      16, // 64 KiB pieces
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
			want:           16, // 64 KiB pieces (minimum)
		},
		{
			name:           "max piece length above maximum should use maximum",
			totalSize:      1 << 40,
			maxPieceLength: uint_ptr(30),
			want:           24, // 16 MiB pieces
		},
		{
			name:         "target 1000 pieces for 1GB file (best-effort)",
			totalSize:    1 << 30, // 1 GiB
			piecesTarget: uint_ptr(1000),
			want:         20, // 1 MiB pieces gives ~1024 pieces (closest power of 2)
			wantPieces:   uint_ptr(1024),
		},
		{
			name:         "target 1000 pieces for 10GB file (best-effort)",
			totalSize:    10 << 30, // 10 GiB
			piecesTarget: uint_ptr(1000),
			want:         23, // 8 MiB pieces gives ~1280 pieces (closest we can get within bounds)
			wantPieces:   uint_ptr(1280),
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
			want:         16, // 64 KiB pieces, ~16 pieces
			wantPieces:   uint_ptr(16),
		},
		{
			name:         "zero target pieces should use default calculation",
			totalSize:    1 << 30, // 1 GiB
			piecesTarget: uint_ptr(0),
			want:         21, // 2 MiB pieces
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
		{
			name:      "58 MiB file should use 64 KiB pieces",
			totalSize: 58 << 20,
			want:      16,
		},
		{
			name:      "122 MiB file should use 128 KiB pieces",
			totalSize: 122 << 20,
			want:      17,
		},
		{
			name:      "213 MiB file should use 256 KiB pieces",
			totalSize: 213 << 20,
			want:      18,
		},
		{
			name:      "444 MiB file should use 512 KiB pieces",
			totalSize: 444 << 20,
			want:      19,
		},
		{
			name:      "922 MiB file should use 1 MiB pieces",
			totalSize: 922 << 20,
			want:      20,
		},
		{
			name:      "3.88 GiB file should use 2 MiB pieces",
			totalSize: 3977 << 20,
			want:      21,
		},
		{
			name:      "6.70 GiB file should use 4 MiB pieces",
			totalSize: 6861 << 20,
			want:      22,
		},
		{
			name:      "13.90 GiB file should use 8 MiB pieces",
			totalSize: 14234 << 20,
			want:      23,
		},
		{
			name:       "emp should respect max piece length of 2^23",
			totalSize:  100 << 30, // 100 GiB
			trackerURL: "https://empornium.sx/announce?passkey=123",
			want:       23, // limited to 8 MiB pieces
		},
		{
			name:           "emp should use most restrictive limit between tracker and user",
			totalSize:      100 << 30, // 100 GiB
			trackerURL:     "https://empornium.sx/announce?passkey=123",
			maxPieceLength: uint_ptr(22), // user wants 4 MiB pieces
			want:           22,           // user's limit is more restrictive
		},
		{
			name:           "emp should ignore user's higher max piece length",
			totalSize:      100 << 30, // 100 GiB
			trackerURL:     "https://empornium.sx/announce?passkey=123",
			maxPieceLength: uint_ptr(24), // user wants 16 MiB pieces
			want:           23,           // tracker's limit of 8 MiB pieces wins
		},
		{
			name:       "ptp should aim for 1000 pieces (best-effort)",
			totalSize:  10 << 30, // 10 GiB
			trackerURL: "https://please.passthe.tea/announce?passkey=123",
			want:       23, // 8 MiB pieces gives ~1280 pieces (closest we can get within bounds)
			wantPieces: uint_ptr(1280),
		},
		{
			name:         "user piece target should override ptp default (best-effort)",
			totalSize:    10 << 30, // 10 GiB
			trackerURL:   "https://please.passthe.tea/announce?passkey=123",
			piecesTarget: uint_ptr(2000),
			want:         22, // 4 MiB pieces gives ~2560 pieces (closest power of 2 within bounds)
			wantPieces:   uint_ptr(2560),
		},
		{
			name:       "unknown tracker should use default calculation",
			totalSize:  10 << 30, // 10 GiB
			trackerURL: "https://unknown.tracker.com/announce",
			want:       23, // 8 MiB pieces
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePieceLength(tt.totalSize, tt.maxPieceLength, tt.piecesTarget, tt.trackerURL)
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
