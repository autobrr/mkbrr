package trackers

import (
	"testing"
)

func Test_GetTrackerPieceSizeExp(t *testing.T) {
	tests := []struct {
		name        string
		trackerURL  string
		contentSize uint64
		wantExp     uint
		wantFound   bool
	}{
		{
			name:        "ggn small file should use 32 KiB pieces",
			trackerURL:  "https://gazellegames.net/announce?passkey=123",
			contentSize: 32 << 20, // 32 MB
			wantExp:     15,       // 32 KiB pieces
			wantFound:   true,
		},
		{
			name:        "ggn medium file should use 1 MiB pieces",
			trackerURL:  "https://gazellegames.net/announce?passkey=123",
			contentSize: (3 << 29), // 1.5 GB (3 * 512MB)
			wantExp:     20,        // 1 MiB pieces
			wantFound:   true,
		},
		{
			name:        "ggn huge file should use 64 MiB pieces",
			trackerURL:  "https://gazellegames.net/announce?passkey=123",
			contentSize: 100 << 30, // 100 GB
			wantExp:     26,        // 64 MiB pieces
			wantFound:   true,
		},
		{
			name:        "unknown tracker should not return piece size recommendations",
			trackerURL:  "https://unknown.tracker/announce",
			contentSize: 1 << 30,
			wantExp:     0,
			wantFound:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExp, gotFound := GetTrackerPieceSizeExp(tt.trackerURL, tt.contentSize)
			if gotFound != tt.wantFound {
				t.Errorf("GetTrackerPieceSizeExp() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotExp != tt.wantExp {
				t.Errorf("GetTrackerPieceSizeExp() exp = %v, want %v", gotExp, tt.wantExp)
			}
		})
	}
}

func Test_GetTrackerMaxPieceLength(t *testing.T) {
	tests := []struct {
		name       string
		trackerURL string
		wantExp    uint
		wantFound  bool
	}{
		{
			name:       "ggn should allow up to 64 MiB pieces",
			trackerURL: "https://gazellegames.net/announce?passkey=123",
			wantExp:    26, // 64 MiB pieces
			wantFound:  true,
		},
		{
			name:       "ptp should allow up to 16 MiB pieces",
			trackerURL: "https://passthepopcorn.me/announce?passkey=123",
			wantExp:    24, // 16 MiB pieces
			wantFound:  true,
		},
		{
			name:       "hdb should allow up to 16 MiB pieces",
			trackerURL: "https://hdbits.org/announce?passkey=123",
			wantExp:    24, // 16 MiB pieces
			wantFound:  true,
		},
		{
			name:       "emp should allow up to 8 MiB pieces",
			trackerURL: "https://empornium.sx/announce?passkey=123",
			wantExp:    23, // 8 MiB pieces
			wantFound:  true,
		},
		{
			name:       "mtv should allow up to 8 MiB pieces",
			trackerURL: "https://morethantv.me/announce?passkey=123",
			wantExp:    23, // 8 MiB pieces
			wantFound:  true,
		},
		{
			name:       "unknown tracker should not return max piece length",
			trackerURL: "https://unknown.tracker/announce",
			wantExp:    0,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExp, gotFound := GetTrackerMaxPieceLength(tt.trackerURL)
			if gotFound != tt.wantFound {
				t.Errorf("GetTrackerMaxPieceLength() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotExp != tt.wantExp {
				t.Errorf("GetTrackerMaxPieceLength() exp = %v, want %v", gotExp, tt.wantExp)
			}
		})
	}
}

func Test_GetTrackerMaxTorrentSize(t *testing.T) {
	tests := []struct {
		name       string
		trackerURL string
		wantSize   uint64
		wantFound  bool
	}{
		{
			name:       "ggn should have 1 MB torrent size limit",
			trackerURL: "https://gazellegames.net/announce?passkey=123",
			wantSize:   1 << 20, // 1 MB
			wantFound:  true,
		},
		{
			name:       "anthelion should have 250 KiB torrent size limit",
			trackerURL: "https://anthelion.me/announce?passkey=123",
			wantSize:   250 << 10, // 250 KiB torrent file size limit
			wantFound:  true,
		},
		{
			name:       "ptp should not have torrent size limit",
			trackerURL: "https://passthepopcorn.me/announce?passkey=123",
			wantSize:   0,
			wantFound:  false,
		},
		{
			name:       "hdb should not have torrent size limit",
			trackerURL: "https://hdbits.org/announce?passkey=123",
			wantSize:   0,
			wantFound:  false,
		},
		{
			name:       "unknown tracker should not have torrent size limit",
			trackerURL: "https://unknown.tracker/announce",
			wantSize:   0,
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSize, gotFound := GetTrackerMaxTorrentSize(tt.trackerURL)
			if gotFound != tt.wantFound {
				t.Errorf("GetTrackerMaxTorrentSize() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotSize != tt.wantSize {
				t.Errorf("GetTrackerMaxTorrentSize() size = %v, want %v", gotSize, tt.wantSize)
			}
		})
	}
}

func Test_trackerConfigConsistency(t *testing.T) {
	for _, config := range trackerConfigs {
		// Skip empty configs
		if len(config.URLs) == 0 {
			t.Error("found tracker config with no URLs")
			continue
		}

		// Verify piece size ranges are in ascending order
		for i := 1; i < len(config.PieceSizeRanges); i++ {
			if config.PieceSizeRanges[i].MaxSize <= config.PieceSizeRanges[i-1].MaxSize {
				t.Errorf("tracker %v: piece size range %d (max size %d) is not greater than range %d (max size %d)",
					config.URLs, i, config.PieceSizeRanges[i].MaxSize, i-1, config.PieceSizeRanges[i-1].MaxSize)
			}
		}

		// Verify piece size exponents are within bounds
		for i, r := range config.PieceSizeRanges {
			if r.PieceExp > config.MaxPieceLength {
				t.Errorf("tracker %v: piece size range %d has exponent %d exceeding max piece length %d",
					config.URLs, i, r.PieceExp, config.MaxPieceLength)
			}
		}

		// Verify piece size ranges don't have gaps
		if len(config.PieceSizeRanges) > 0 {
			for i := 1; i < len(config.PieceSizeRanges); i++ {
				prev := config.PieceSizeRanges[i-1]
				curr := config.PieceSizeRanges[i]

				// skip check if current range is the "infinity" range
				if curr.MaxSize == ^uint64(0) {
					continue
				}

				// verify current range starts where previous range ends
				if curr.MaxSize <= prev.MaxSize {
					t.Errorf("tracker %v: piece size range %d (max size %d) must be greater than range %d (max size %d)",
						config.URLs, i, curr.MaxSize, i-1, prev.MaxSize)
				}
			}
		}
	}
}
