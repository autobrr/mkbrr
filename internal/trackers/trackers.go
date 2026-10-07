// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package trackers

import (
	"slices"
	"strings"
)

// Rules holds the tracker rules that mkbrr enforces for one tracker
type Rules struct {
	DefaultSource    string           // default source to use for this tracker
	URLs             []string         // list of tracker URLs that share this config
	PieceSizeRanges  []PieceSizeRange // custom piece size ranges for specific content sizes
	MaxPieceLength   uint             // maximum piece length exponent (2^n). default is 24 (16 MiB) from create.go
	MaxTorrentSize   uint64           // maximum .torrent file size in bytes (0 means no limit)
	UseDefaultRanges bool             // whether to use default piece size ranges when content size is outside custom ranges
}

// PieceSizeRange defines a range of content sizes and their corresponding piece size exponent
type PieceSizeRange struct {
	MaxSize  uint64 // maximum content size in bytes for this range
	PieceExp uint   // piece size exponent (2^n)
}

// trackerConfigs maps known tracker base URLs to their configurations
var trackerConfigs = []Rules{
	{
		URLs: []string{
			"anthelion.me",
		},
		MaxTorrentSize: 250 << 10, // 250 KiB torrent file size limit
		DefaultSource:  "ANT",
	},
	{
		URLs: []string{
			"nebulance.io",
		},
		MaxTorrentSize: 1024 << 10, // 1 MiB torrent file size limit
		DefaultSource:  "NBL",
	},
	{
		URLs: []string{
			"hdbits.org",
			"superbits.org",
			"sptracker.cc",
		},
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: true,
	},
	{
		URLs: []string{
			"beyond-hd.me",
		},
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: true,
		DefaultSource:    "BHD",
	},
	{
		URLs: []string{
			"passthepopcorn.me",
		}, // https://ptp/upload.php?action=piecesize
		MaxPieceLength: 24, // max 16 MiB pieces (2^24)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 58 << 20, PieceExp: 16},    // 64 KiB for <= 58 MiB
			{MaxSize: 122 << 20, PieceExp: 17},   // 128 KiB for 58-122 MiB
			{MaxSize: 213 << 20, PieceExp: 18},   // 256 KiB for 122-213 MiB
			{MaxSize: 444 << 20, PieceExp: 19},   // 512 KiB for 213-444 MiB
			{MaxSize: 922 << 20, PieceExp: 20},   // 1 MiB for 444-922 MiB
			{MaxSize: 3977 << 20, PieceExp: 21},  // 2 MiB for 922 MiB-3.88 GiB
			{MaxSize: 6861 << 20, PieceExp: 22},  // 4 MiB for 3.88-6.70 GiB
			{MaxSize: 14234 << 20, PieceExp: 23}, // 8 MiB for 6.70-13.90 GiB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 13.90 GiB
		},
		UseDefaultRanges: false,
		DefaultSource:    "PTP",
	},
	{
		URLs: []string{
			"morethantv.me", // https://mtv/forum/thread/3237?postid=74725#post74725
		},
		MaxPieceLength:   23, // max 8 MiB pieces (2^23)
		UseDefaultRanges: true,
		DefaultSource:    "MTV",
	},
	{
		URLs: []string{
			"empornium.sx",
		},
		MaxPieceLength:   23, // max 8 MiB pieces (2^23)
		UseDefaultRanges: true,
		DefaultSource:    "Emp",
	},
	{
		URLs: []string{
			"gazellegames.net",
		},
		MaxPieceLength: 26, // max 64 MiB pieces (2^26)
		PieceSizeRanges: []PieceSizeRange{ // https://ggn/wiki.php?action=article&id=300
			{MaxSize: 64 << 20, PieceExp: 15},    // 32 KiB for < 64 MB
			{MaxSize: 128 << 20, PieceExp: 16},   // 64 KiB for 64-128 MB
			{MaxSize: 256 << 20, PieceExp: 17},   // 128 KiB for 128-256 MB
			{MaxSize: 512 << 20, PieceExp: 18},   // 256 KiB for 256-512 MB
			{MaxSize: 1024 << 20, PieceExp: 19},  // 512 KiB for 512 MB-1 GB
			{MaxSize: 2048 << 20, PieceExp: 20},  // 1 MiB for 1-2 GB
			{MaxSize: 4096 << 20, PieceExp: 21},  // 2 MiB for 2-4 GB
			{MaxSize: 8192 << 20, PieceExp: 22},  // 4 MiB for 4-8 GB
			{MaxSize: 16384 << 20, PieceExp: 23}, // 8 MiB for 8-16 GB
			{MaxSize: 32768 << 20, PieceExp: 24}, // 16 MiB for 16-32 GB
			{MaxSize: 65536 << 20, PieceExp: 25}, // 32 MiB for 32-64 GB
			{MaxSize: ^uint64(0), PieceExp: 26},  // 64 MiB for > 64 GB
		},
		UseDefaultRanges: false,
		MaxTorrentSize:   1 << 20, // 1 MB torrent file size limit
		DefaultSource:    "GGn",
	},
	{
		URLs: []string{
			"tracker.alpharatio.cc",
		},
		MaxPieceLength: 26, // max 64 MiB pieces (2^26)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 64 << 20, PieceExp: 15},    // 32 KiB for < 64 MB
			{MaxSize: 128 << 20, PieceExp: 16},   // 64 KiB for 64-128 MB
			{MaxSize: 256 << 20, PieceExp: 17},   // 128 KiB for 128-256 MB
			{MaxSize: 512 << 20, PieceExp: 18},   // 256 KiB for 256-512 MB
			{MaxSize: 1024 << 20, PieceExp: 19},  // 512 KiB for 512 MB-1 GB
			{MaxSize: 2048 << 20, PieceExp: 20},  // 1 MiB for 1-2 GB
			{MaxSize: 4096 << 20, PieceExp: 21},  // 2 MiB for 2-4 GB
			{MaxSize: 8192 << 20, PieceExp: 22},  // 4 MiB for 4-8 GB
			{MaxSize: 16384 << 20, PieceExp: 23}, // 8 MiB for 8-16 GB
			{MaxSize: 32768 << 20, PieceExp: 24}, // 16 MiB for 16-32 GB
			{MaxSize: 65536 << 20, PieceExp: 25}, // 32 MiB for 32-64 GB
			{MaxSize: ^uint64(0), PieceExp: 26},  // 64 MiB for > 64 GB
		},
		UseDefaultRanges: false,
		MaxTorrentSize:   2 << 20, // 2 MB torrent file size limit
		DefaultSource:    "AlphaRatio",
	},
	{
		URLs: []string{
			"seedpool.org",
		},
		MaxPieceLength: 27, // max 128 MiB pieces (2^27)
		PieceSizeRanges: []PieceSizeRange{ // Mirror default calculation logic from create.go
			{MaxSize: 64 << 20, PieceExp: 15},     // 32 KiB for <= 64 MB
			{MaxSize: 128 << 20, PieceExp: 16},    // 64 KiB for 64-128 MB
			{MaxSize: 256 << 20, PieceExp: 17},    // 128 KiB for 128-256 MB
			{MaxSize: 512 << 20, PieceExp: 18},    // 256 KiB for 256-512 MB
			{MaxSize: 1024 << 20, PieceExp: 19},   // 512 KiB for 512 MB-1 GB
			{MaxSize: 2048 << 20, PieceExp: 20},   // 1 MiB for 1-2 GB
			{MaxSize: 4096 << 20, PieceExp: 21},   // 2 MiB for 2-4 GB
			{MaxSize: 8192 << 20, PieceExp: 22},   // 4 MiB for 4-8 GB
			{MaxSize: 16384 << 20, PieceExp: 23},  // 8 MiB for 8-16 GB
			{MaxSize: 32768 << 20, PieceExp: 24},  // 16 MiB for 16-32 GB
			{MaxSize: 65536 << 20, PieceExp: 25},  // 32 MiB for 32-64 GB
			{MaxSize: 131072 << 20, PieceExp: 26}, // 64 MiB for 64-128 GB
			{MaxSize: ^uint64(0), PieceExp: 27},   // 128 MiB for > 128 GB
		},
		UseDefaultRanges: false,
		DefaultSource:    "seedpool.org",
	},
	{
		URLs: []string{
			"norbits.net",
		},
		PieceSizeRanges: []PieceSizeRange{ // https://nb/ulguide.php
			{MaxSize: 250 << 20, PieceExp: 18},   // 256 KiB for < 250 MB
			{MaxSize: 1024 << 20, PieceExp: 20},  // 1 MiB for 250-1024 MB
			{MaxSize: 5120 << 20, PieceExp: 21},  // 2 MiB for 1-5 GB
			{MaxSize: 20480 << 20, PieceExp: 22}, // 4 MiB for 5-20 GB
			{MaxSize: 40960 << 20, PieceExp: 23}, // 8 MiB for 20-40 GB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 40 GB
		},
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: false,
	},
	{
		URLs: []string{
			"landof.tv",
		},
		PieceSizeRanges: []PieceSizeRange{ // https://btn/forums.php?action=viewthread&threadid=18301
			{MaxSize: 32 << 20, PieceExp: 15},   // 32 KiB for <= 32 MiB
			{MaxSize: 62 << 20, PieceExp: 16},   // 64 KiB for 32-62 MiB
			{MaxSize: 125 << 20, PieceExp: 17},  // 128 KiB for 62-125 MiB
			{MaxSize: 250 << 20, PieceExp: 18},  // 256 KiB for 125-250 MiB
			{MaxSize: 500 << 20, PieceExp: 19},  // 512 KiB for 250-500 MiB
			{MaxSize: 1000 << 20, PieceExp: 20}, // 1 MiB for 500-1000 MiB
			{MaxSize: 1945 << 20, PieceExp: 21}, // 2 MiB for 1000 MiB-1.95 GiB
			{MaxSize: 3906 << 20, PieceExp: 22}, // 4 MiB for 1.95-3.906 GiB
			{MaxSize: 7810 << 20, PieceExp: 23}, // 8 MiB for 3.906-7.81 GiB
			{MaxSize: ^uint64(0), PieceExp: 24}, // 16 MiB for > 7.81 GiB
		},
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: false,
	},
	{
		URLs: []string{
			"torrent-syndikat.org",
			"tee-stube.org",
		},
		MaxPieceLength: 24, // max 16 MiB pieces (2^24)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 250 << 20, PieceExp: 20},   // 1 MiB for < 250 MB
			{MaxSize: 1024 << 20, PieceExp: 20},  // 1 MiB for 250 MB-1 GB
			{MaxSize: 5120 << 20, PieceExp: 20},  // 1 MiB for 1-5 GB
			{MaxSize: 20480 << 20, PieceExp: 22}, // 4 MiB for 5-20 GB
			{MaxSize: 51200 << 20, PieceExp: 23}, // 8 MiB for 20-50 GB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 50 GB
		},
		UseDefaultRanges: false,
	},
	{
		URLs: []string{
			"onlyencodes.cc",
		},
		MaxPieceLength: 24, // max 16 MiB pieces (2^24)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 1024 << 20, PieceExp: 20},  // 1 MiB < 1 GB
			{MaxSize: 4096 << 20, PieceExp: 21},  // 2 MiB for 1-4 GB
			{MaxSize: 12288 << 20, PieceExp: 22}, // 4 MiB for 4-12 GB
			{MaxSize: 20480 << 20, PieceExp: 23}, // 8 MiB for 12-20 GB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 20 GB
		},
		UseDefaultRanges: false,
	},
	{
		URLs: []string{
			"portugas.org",
		},
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 40 << 20, PieceExp: 14},   // 16 KiB for <= 40 MiB
			{MaxSize: 70 << 20, PieceExp: 15},   // 32 KiB for 40-70 MiB
			{MaxSize: 150 << 20, PieceExp: 16},  // 64 KiB for 70-150 MiB
			{MaxSize: 300 << 20, PieceExp: 17},  // 128 KiB for 150-300 MiB
			{MaxSize: 600 << 20, PieceExp: 18},  // 256 KiB for 300-600 MiB
			{MaxSize: 1 << 30, PieceExp: 19},    // 512 KiB for 600 MiB-1 GiB
			{MaxSize: 2304 << 20, PieceExp: 20}, // 1 MiB for 1-2.25 GiB
			{MaxSize: 5 << 30, PieceExp: 21},    // 2 MiB for 2.25-5 GiB
			{MaxSize: 8 << 30, PieceExp: 22},    // 4 MiB for 5-8 GiB
			{MaxSize: 16 << 30, PieceExp: 23},   // 8 MiB for 8-16 GiB
			{MaxSize: 35 << 30, PieceExp: 24},   // 16 MiB for 16-35 GiB
			{MaxSize: 72 << 30, PieceExp: 25},   // 32 MiB for 35-72 GiB
			{MaxSize: ^uint64(0), PieceExp: 25}, // 32+ MiB above 72 GiB
		},
		UseDefaultRanges: false,
		MaxTorrentSize:   2 << 20, // 2 MiB .torrent file size limit
	},
	{
		URLs: []string{
			"lst.gg",
		},
		MaxPieceLength: 24, // max 16 MiB pieces (2^24)
		PieceSizeRanges: []PieceSizeRange{ // https://lst/pages/8
			{MaxSize: 1024 << 20, PieceExp: 20},  // 1 MiB < 1 GB
			{MaxSize: 4096 << 20, PieceExp: 21},  // 2 MiB for 1-4 GB
			{MaxSize: 12288 << 20, PieceExp: 22}, // 4 MiB for 4-12 GB
			{MaxSize: 20480 << 20, PieceExp: 23}, // 8 MiB for 12-20 GB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 20 GB
		},
		UseDefaultRanges: false,
		DefaultSource:    "lst.gg",
	},
	{
		URLs: []string{
			"aither.cc",
		},
		MaxPieceLength: 27, // max 128 MiB pieces (2^27) (only when set with --piece-size)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 1024 << 20, PieceExp: 20},  // 1 MiB < 1 GB
			{MaxSize: 4096 << 20, PieceExp: 21},  // 2 MiB for 1-4 GB
			{MaxSize: 12288 << 20, PieceExp: 22}, // 4 MiB for 4-12 GB
			{MaxSize: 20480 << 20, PieceExp: 23}, // 8 MiB for 12-20 GB
			{MaxSize: ^uint64(0), PieceExp: 24},  // 16 MiB for > 20 GB
		},
		UseDefaultRanges: false,
		DefaultSource:    "Aither",
	},
	{
		URLs: []string{
			"upload.cx",
		},
		DefaultSource: "ULCX",
	},
	{
		URLs: []string{
			"capybarabr.com",
		},
		DefaultSource: "CapybaraBR",
	},
	{
		URLs: []string{
			"hawke.uno",
		},
		DefaultSource: "HUNO",
	},
	{
		URLs: []string{
			"tracker.torrentleech.org",
			"tracker.tleechreload.org",
		},
		MaxPieceLength: 27, // max 128 MiB pieces (2^27)
		PieceSizeRanges: []PieceSizeRange{
			{MaxSize: 50 << 20, PieceExp: 15},     // 32 KiB for <= 50 MiB
			{MaxSize: 150 << 20, PieceExp: 16},    // 64 KiB for 50-150 MiB
			{MaxSize: 350 << 20, PieceExp: 17},    // 128 KiB for 150-350 MiB
			{MaxSize: 512 << 20, PieceExp: 18},    // 256 KiB for 350-512 MiB
			{MaxSize: 1024 << 20, PieceExp: 19},   // 512 KiB for 512 MiB-1 GiB
			{MaxSize: 2048 << 20, PieceExp: 20},   // 1 MiB for 1-2 GiB
			{MaxSize: 4096 << 20, PieceExp: 21},   // 2 MiB for 2-4 GiB
			{MaxSize: 8192 << 20, PieceExp: 22},   // 4 MiB for 4-8 GiB
			{MaxSize: 16384 << 20, PieceExp: 23},  // 8 MiB for 8-16 GiB
			{MaxSize: 32768 << 20, PieceExp: 24},  // 16 MiB for 16-32 GiB
			{MaxSize: 65536 << 20, PieceExp: 25},  // 32 MiB for 32-64 GiB
			{MaxSize: 122880 << 20, PieceExp: 26}, // 64 MiB for 64-120 GiB
			{MaxSize: ^uint64(0), PieceExp: 27},   // 128 MiB for 120+ GiB (TL supports 2^28 but mkbrr caps at 2^27)
		},
		UseDefaultRanges: false,
		DefaultSource:    "TorrentLeech.org",
	},
}

// Lookup returns the rules for a tracker URL.
func Lookup(trackerURL string) (Rules, bool) {
	for _, r := range trackerConfigs {
		for _, url := range r.URLs {
			if strings.Contains(trackerURL, url) {
				return r, true
			}
		}
	}
	return Rules{}, false
}

// DefaultPieceSizeRanges defines the default piece size calculation ranges
// Used by both the trackers package and torrent/create.go for automatic piece length
var DefaultPieceSizeRanges = []PieceSizeRange{
	{MaxSize: 64 << 20, PieceExp: 15},     // 32 KiB for <= 64 MB
	{MaxSize: 128 << 20, PieceExp: 16},    // 64 KiB for 64-128 MB
	{MaxSize: 256 << 20, PieceExp: 17},    // 128 KiB for 128-256 MB
	{MaxSize: 512 << 20, PieceExp: 18},    // 256 KiB for 256-512 MB
	{MaxSize: 1024 << 20, PieceExp: 19},   // 512 KiB for 512 MB-1 GB
	{MaxSize: 2048 << 20, PieceExp: 20},   // 1 MiB for 1-2 GB
	{MaxSize: 4096 << 20, PieceExp: 21},   // 2 MiB for 2-4 GB
	{MaxSize: 8192 << 20, PieceExp: 22},   // 4 MiB for 4-8 GB
	{MaxSize: 16384 << 20, PieceExp: 23},  // 8 MiB for 8-16 GB
	{MaxSize: 32768 << 20, PieceExp: 24},  // 16 MiB for 16-32 GB
	{MaxSize: 65536 << 20, PieceExp: 25},  // 32 MiB for 32-64 GB
	{MaxSize: 131072 << 20, PieceExp: 26}, // 64 MiB for 64-128 GB
	{MaxSize: ^uint64(0), PieceExp: 27},   // 128 MiB for > 128 GB
}

// PieceSizeExp returns the recommended piece size exponent for a content size,
// clamped to MaxPieceLength.
func (r Rules) PieceSizeExp(contentSize uint64) (uint, bool) {
	ranges := r.PieceSizeRanges
	if len(ranges) == 0 && r.UseDefaultRanges {
		ranges = DefaultPieceSizeRanges
	}
	if len(ranges) == 0 {
		return 0, false
	}

	exp := ranges[len(ranges)-1].PieceExp
	for _, pr := range ranges {
		if contentSize <= pr.MaxSize {
			exp = pr.PieceExp
			break
		}
	}
	if r.MaxPieceLength > 0 && exp > r.MaxPieceLength {
		exp = r.MaxPieceLength
	}
	return exp, true
}

// Configs returns a copy of the rules for every known tracker.
func Configs() []Rules {
	return slices.Clone(trackerConfigs)
}
