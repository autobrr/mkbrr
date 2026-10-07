// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package trackers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRules_PieceSizeExp(t *testing.T) {
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
			name:        "torrent-syndikat small file should use 1 MiB pieces",
			trackerURL:  "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			contentSize: 200 << 20, // 200 MB
			wantExp:     20,        // 1 MiB pieces
			wantFound:   true,
		},
		{
			name:        "torrent-syndikat medium file should use 1 MiB pieces",
			trackerURL:  "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			contentSize: 3 << 30, // 3 GB
			wantExp:     20,      // 1 MiB pieces
			wantFound:   true,
		},
		{
			name:        "torrent-syndikat large file should use 4 MiB pieces",
			trackerURL:  "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			contentSize: 10 << 30, // 10 GB
			wantExp:     22,       // 4 MiB pieces
			wantFound:   true,
		},
		{
			name:        "torrent-syndikat very large file should use 8 MiB pieces",
			trackerURL:  "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			contentSize: 40 << 30, // 40 GB
			wantExp:     23,       // 8 MiB pieces
			wantFound:   true,
		},
		{
			name:        "torrent-syndikat huge file should use 16 MiB pieces",
			trackerURL:  "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			contentSize: 60 << 30, // 60 GB
			wantExp:     24,       // 16 MiB pieces
			wantFound:   true,
		},
		{
			name:        "torrent-syndikat alternate domain should use correct piece size",
			trackerURL:  "https://ulo.tee-stube.org/ts_ann.php?passkey=123",
			contentSize: 60 << 30, // 60 GB
			wantExp:     24,       // 16 MiB pieces
			wantFound:   true,
		},
		{
			name:        "onlyencodes should use lst piece rules",
			trackerURL:  "https://onlyencodes.cc/announce?passkey=123",
			contentSize: 8 << 30, // 8 GB
			wantExp:     22,      // 4 MiB pieces
			wantFound:   true,
		},
		{
			name:        "portugas <=40MiB should recommend 16 KiB pieces",
			trackerURL:  "https://portugas.org/announce/passkey",
			contentSize: 40 << 20,
			wantExp:     14,
			wantFound:   true,
		},
		{
			name:        "portugas 60MiB should use 32 KiB pieces",
			trackerURL:  "https://portugas.org/announce/passkey",
			contentSize: 60 << 20,
			wantExp:     15,
			wantFound:   true,
		},
		{
			name:        "portugas 12GiB should use 8 MiB pieces",
			trackerURL:  "https://portugas.org/announce/passkey",
			contentSize: 12 << 30,
			wantExp:     23,
			wantFound:   true,
		},
		{
			name:        "portugas 40GiB should use 32 MiB pieces",
			trackerURL:  "https://portugas.org/announce/passkey",
			contentSize: 40 << 30,
			wantExp:     25,
			wantFound:   true,
		},
		{
			name:        "portugas above 72GiB should start at 32 MiB pieces",
			trackerURL:  "https://portugas.org/announce/passkey",
			contentSize: 100 << 30,
			wantExp:     25,
			wantFound:   true,
		},
		{
			name:        "unknown tracker should not return piece size recommendations",
			trackerURL:  "https://unknown.tracker/announce",
			contentSize: 1 << 30,
			wantExp:     0,
			wantFound:   false,
		},
		{
			name:        "bhd small file should use default 32 KiB pieces",
			trackerURL:  "https://beyond-hd.me/announce?passkey=123",
			contentSize: 32 << 20, // 32 MB
			wantExp:     15,       // 32 KiB pieces (from default ranges)
			wantFound:   true,
		},
		{
			name:        "bhd medium file should use default 2 MiB pieces",
			trackerURL:  "https://beyond-hd.me/announce?passkey=123",
			contentSize: 3 << 30, // 3 GB
			wantExp:     21,      // 2 MiB pieces (from default ranges)
			wantFound:   true,
		},
		{
			name:        "bhd large file should be clamped to max 16 MiB pieces",
			trackerURL:  "https://beyond-hd.me/announce?passkey=123",
			contentSize: 50 << 30, // 50 GB
			wantExp:     24,       // 16 MiB pieces (clamped from default 32 MiB)
			wantFound:   true,
		},
		{
			name:        "hdb should use default ranges with max clamping",
			trackerURL:  "https://hdbits.org/announce?passkey=123",
			contentSize: 50 << 30, // 50 GB
			wantExp:     24,       // 16 MiB pieces (clamped from default 32 MiB)
			wantFound:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, _ := Lookup(tt.trackerURL)
			gotExp, gotFound := rules.PieceSizeExp(tt.contentSize)
			if gotFound != tt.wantFound {
				t.Errorf("PieceSizeExp() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotExp != tt.wantExp {
				t.Errorf("PieceSizeExp() exp = %v, want %v", gotExp, tt.wantExp)
			}
		})
	}
}

func TestLookup_CustomPieceSizeRanges(t *testing.T) {
	tests := []struct {
		name       string
		trackerURL string
		want       bool
	}{
		{
			name:       "ggn has a custom range table",
			trackerURL: "https://gazellegames.net/announce?passkey=123",
			want:       true,
		},
		{
			name:       "bhd uses default ranges rather than a custom table",
			trackerURL: "https://beyond-hd.me/announce?passkey=123",
			want:       false,
		},
		{
			name:       "unknown tracker has no custom range table",
			trackerURL: "https://unknown.tracker/announce",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, _ := Lookup(tt.trackerURL)
			assert.Equal(t, tt.want, len(rules.PieceSizeRanges) > 0)
		})
	}
}

func TestLookup_MaxPieceLength(t *testing.T) {
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
			name:       "torrent-syndikat should allow up to 16 MiB pieces",
			trackerURL: "https://ulo.torrent-syndikat.org/ts_ann.php?passkey=123",
			wantExp:    24, // 16 MiB pieces
			wantFound:  true,
		},
		{
			name:       "torrent-syndikat alternate domain should allow up to 16 MiB pieces",
			trackerURL: "https://ulo.tee-stube.org/ts_ann.php?passkey=123",
			wantExp:    24, // 16 MiB pieces
			wantFound:  true,
		},
		{
			name:       "onlyencodes should allow up to 16 MiB pieces",
			trackerURL: "https://onlyencodes.cc/announce?passkey=123",
			wantExp:    24, // 16 MiB pieces
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
			rules, gotFound := Lookup(tt.trackerURL)
			gotExp := rules.MaxPieceLength
			if gotFound != tt.wantFound {
				t.Errorf("Lookup() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotExp != tt.wantExp {
				t.Errorf("Lookup() exp = %v, want %v", gotExp, tt.wantExp)
			}
		})
	}
}

func TestLookup_MaxTorrentSize(t *testing.T) {
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
			name:       "portugas should have 2 MiB torrent size limit",
			trackerURL: "https://portugas.org/announce/passkey",
			wantSize:   2 << 20,
			wantFound:  true,
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
			rules, _ := Lookup(tt.trackerURL)
			gotSize, gotFound := rules.MaxTorrentSize, rules.MaxTorrentSize > 0
			if gotFound != tt.wantFound {
				t.Errorf("Lookup() found = %v, want %v", gotFound, tt.wantFound)
			}
			if gotSize != tt.wantSize {
				t.Errorf("Lookup() size = %v, want %v", gotSize, tt.wantSize)
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
			if config.MaxPieceLength > 0 && r.PieceExp > config.MaxPieceLength {
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
