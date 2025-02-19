package torrent

import "strings"

// TrackerConfig holds tracker-specific configuration
type TrackerConfig struct {
	PiecesTarget     uint             // target number of pieces (best-effort, will try to get as close as possible within power-of-2 constraints)
	MaxPieceLength   uint             // maximum piece length exponent (2^n)
	PieceSizeRanges  []PieceSizeRange // custom piece size ranges for specific content sizes
	UseDefaultRanges bool             // whether to use default piece size ranges when content size is outside custom ranges
}

// PieceSizeRange defines a range of content sizes and their corresponding piece size exponent
type PieceSizeRange struct {
	MaxSize  uint64 // maximum content size in bytes for this range
	PieceExp uint   // piece size exponent (2^n)
}

// trackerConfigs maps known tracker base URLs to their configurations
var trackerConfigs = map[string]TrackerConfig{
	"passthepopcorn.me": {
		MaxPieceLength:   24, // max 16 MiB pieces (2^24)
		UseDefaultRanges: true,
	},
	"empornium.sx": {
		MaxPieceLength:   23, // max 8 MiB pieces (2^23)
		UseDefaultRanges: true,
	},
	"morethantv.me": {
		MaxPieceLength:   23, // max 8 MiB pieces (2^23)
		UseDefaultRanges: true,
	},
	"gazellegames.net": {
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
	},
}

// GetTrackerPiecesTarget returns the preferred piece count for a tracker if known.
// Note: The returned target is a best-effort goal - the actual piece count may differ
// due to power-of-2 piece length constraints and min/max piece length bounds.
func GetTrackerPiecesTarget(trackerURL string) (uint, bool) {
	for baseURL, config := range trackerConfigs {
		if strings.Contains(trackerURL, baseURL) {
			return config.PiecesTarget, config.PiecesTarget > 0
		}
	}
	return 0, false
}

// GetTrackerMaxPieceLength returns the maximum piece length exponent for a tracker if known.
// This is a hard limit that will not be exceeded.
func GetTrackerMaxPieceLength(trackerURL string) (uint, bool) {
	for baseURL, config := range trackerConfigs {
		if strings.Contains(trackerURL, baseURL) {
			return config.MaxPieceLength, config.MaxPieceLength > 0
		}
	}
	return 0, false
}

// GetTrackerPieceSizeExp returns the recommended piece size exponent for a given content size and tracker
func GetTrackerPieceSizeExp(trackerURL string, contentSize uint64) (uint, bool) {
	for baseURL, config := range trackerConfigs {
		if strings.Contains(trackerURL, baseURL) {
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
	}
	return 0, false
}
