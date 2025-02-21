package torrent

import (
	"testing"
)

func Test_calculatePieceLength(t *testing.T) {
	tests := []struct {
		name           string
		totalSize      int64
		maxPieceLength *uint
		trackerURL     string
		want           uint
		wantPieces     *uint // expected number of pieces (approximate)
	}{
		{
			name:      "small file should use minimum piece length",
			totalSize: 1 << 10, // 1 KiB
			want:      15,      // 32 KiB pieces
		},
		{
			name:      "63MB file should use 32KiB pieces",
			totalSize: 63 << 20,
			want:      15, // 32 KiB pieces
		},
		{
			name:      "65MB file should use 64KiB pieces",
			totalSize: 65 << 20,
			want:      16, // 64 KiB pieces
		},
		{
			name:      "129MB file should use 128KiB pieces",
			totalSize: 129 << 20,
			want:      17, // 128 KiB pieces
		},
		{
			name:      "257MB file should use 256KiB pieces",
			totalSize: 257 << 20,
			want:      18, // 256 KiB pieces
		},
		{
			name:      "513MB file should use 512KiB pieces",
			totalSize: 513 << 20,
			want:      19, // 512 KiB pieces
		},
		{
			name:      "1.1GB file should use 1MiB pieces",
			totalSize: 1100 << 20,
			want:      20, // 1 MiB pieces
		},
		{
			name:      "2.1GB file should use 2MiB pieces",
			totalSize: 2100 << 20,
			want:      21, // 2 MiB pieces
		},
		{
			name:      "4.1GB file should use 4MiB pieces",
			totalSize: 4100 << 20,
			want:      22, // 4 MiB pieces
		},
		{
			name:      "8.1GB file should use 8MiB pieces",
			totalSize: 8200 << 20,
			want:      23, // 8 MiB pieces
		},
		{
			name:      "16.1GB file should use 16MiB pieces",
			totalSize: 16500 << 20,
			want:      24, // 16 MiB pieces
		},
		{
			name:      "256.1GB file should use 16MiB pieces by default",
			totalSize: 256100 << 20, // 256.1 GB
			want:      24,           // 16 MiB pieces
		},
		{
			name:       "emp should respect max piece length of 2^23",
			totalSize:  100 << 30, // 100 GiB
			trackerURL: "https://empornium.sx/announce?passkey=123",
			want:       23, // limited to 8 MiB pieces
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
			got := calculatePieceLength(tt.totalSize, tt.maxPieceLength, tt.trackerURL, false)
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
