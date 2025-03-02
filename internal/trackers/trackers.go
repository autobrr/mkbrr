package trackers

import "strings"

// TrackerConfig holds tracker-specific configuration
type TrackerConfig struct {
	URLs             []string         // list of tracker URLs that share this config
	MaxPieceLength   uint             // maximum piece length exponent (2^n)
	PieceSizeRanges  []PieceSizeRange // custom piece size ranges for specific content sizes
	UseDefaultRanges bool             // whether to use default piece size ranges when content size is outside custom ranges
	MaxTorrentSize   uint64           // maximum .torrent file size in bytes (0 means no limit)
}

// PieceSizeRange defines a range of content sizes and their corresponding piece size exponent
type PieceSizeRange struct {
	MaxSize  uint64 // maximum content size in bytes for this range
	PieceExp uint   // piece size exponent (2^n)
}

// trackerConfigs maps known tracker base URLs to their configurations
var trackerConfigs = []TrackerConfig{
	{
		URLs: []string{
			"anthelion.me",
		},
		MaxTorrentSize: 250 << 10, // 250 KiB torrent file size limit
	},
	{
		URLs: []string{
			"hdbits.org",
			"beyond-hd.me",
			"superbits.org",
			"sptracker.cc",
		},
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: true,
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
	},
	{
		URLs: []string{
			"empornium.sx",
			"morethantv.me", // https://mtv/forum/thread/3237?postid=74725#post74725
		},
		MaxPieceLength:   23, // max 8 MiB pieces (2^23)
		UseDefaultRanges: true,
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
}

// findTrackerConfig returns the config for a given tracker URL
func findTrackerConfig(trackerURL string) *TrackerConfig {
	for i := range trackerConfigs {
		for _, url := range trackerConfigs[i].URLs {
			if strings.Contains(trackerURL, url) {
				return &trackerConfigs[i]
			}
		}
	}
	return nil
}

// GetTrackerMaxPieceLength returns the maximum piece length exponent for a tracker if known.
// This is a hard limit that will not be exceeded.
func GetTrackerMaxPieceLength(trackerURL string) (uint, bool) {
	if config := findTrackerConfig(trackerURL); config != nil {
		return config.MaxPieceLength, config.MaxPieceLength > 0
	}
	return 0, false
}

// GetTrackerPieceSizeExp returns the recommended piece size exponent for a given content size and tracker
func GetTrackerPieceSizeExp(trackerURL string, contentSize uint64) (uint, bool) {
	if config := findTrackerConfig(trackerURL); config != nil {
		if len(config.PieceSizeRanges) > 0 {
			for _, r := range config.PieceSizeRanges {
				if contentSize <= r.MaxSize {
					return r.PieceExp, true
				}
			}
			// if we have ranges but didn't find a match, and UseDefaultRanges is false,
			// use the highest defined piece size
			if !config.UseDefaultRanges {
				return config.PieceSizeRanges[len(config.PieceSizeRanges)-1].PieceExp, true
			}
		}
	}
	return 0, false
}

// GetTrackerMaxTorrentSize returns the maximum allowed .torrent file size for a tracker if known
func GetTrackerMaxTorrentSize(trackerURL string) (uint64, bool) {
	if config := findTrackerConfig(trackerURL); config != nil {
		return config.MaxTorrentSize, config.MaxTorrentSize > 0
	}
	return 0, false
}
