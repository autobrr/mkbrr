package torrentutils

import (
	"fmt"

	"github.com/autobrr/mkbrr/internal/trackers"
	"github.com/autobrr/mkbrr/internal/utils"
)

// PieceLengthConstraints defines the standard constraints for piece length
type PieceLengthConstraints struct {
	MinExp        uint
	MaxExp        uint
	TrackerMaxExp uint
}

// GetPieceLengthConstraints returns the default and tracker-specific constraints for piece length
func getPieceLengthConstraints(trackerURL string) PieceLengthConstraints {
	constraints := PieceLengthConstraints{
		MinExp: 14, // 16 KiB
		MaxExp: 24, // 16 MiB
	}

	// Check if tracker has a maximum piece length constraint
	if trackerURL != "" {
		if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(trackerURL); ok {
			constraints.TrackerMaxExp = trackerMaxExp
		}
	}

	return constraints
}

// ValidatePieceLength checks if a piece length exponent is valid for the given constraints
func ValidatePieceLength(pieceLength uint, trackerURL string) error {
	constraints := getPieceLengthConstraints(trackerURL)

	// Use tracker max if available, otherwise use default max
	maxExp := constraints.MaxExp
	if constraints.TrackerMaxExp > 0 {
		maxExp = constraints.TrackerMaxExp
	}

	if pieceLength < constraints.MinExp || pieceLength > maxExp {
		if trackerURL != "" {
			baseURL := utils.GetDomainPrefix(trackerURL)
			return fmt.Errorf(
				"piece length exponent must be between %d (%s) and %d (%s) for %s, got: %d",
				constraints.MinExp,
				FormatPieceSize(constraints.MinExp),
				maxExp,
				FormatPieceSize(maxExp),
				baseURL,
				pieceLength,
			)
		}
		return fmt.Errorf(
			"piece length exponent must be between %d (%s) and %d (%s), got: %d",
			constraints.MinExp,
			FormatPieceSize(constraints.MinExp),
			maxExp,
			FormatPieceSize(maxExp),
			pieceLength,
		)
	}

	return nil
}

// CalculatePieceLength determines the optimal piece length based on total size
// The min/max bounds take precedence over other constraints
func CalculatePieceLength(totalSize int64, maxPieceLength *uint, trackerURL string) uint {
	constraints := getPieceLengthConstraints(trackerURL)
	minExp := constraints.MinExp
	maxExp := constraints.MaxExp

	// Apply tracker-specific maximum if available
	if trackerURL != "" {
		if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(trackerURL); ok {
			maxExp = trackerMaxExp // Override with tracker's max
		}
	}

	// Check if tracker has specific piece size ranges
	if trackerURL != "" {
		if exp, ok := trackers.GetTrackerPieceSizeExp(trackerURL, uint64(totalSize)); ok {
			// Ensure we stay within bounds
			if exp < minExp {
				exp = minExp
			}
			if exp > maxExp {
				exp = maxExp
			}
			return exp
		}
	}

	// Validate maxPieceLength - if it's below minimum, use minimum
	if maxPieceLength != nil {
		if *maxPieceLength < minExp {
			return minExp
		}
		if *maxPieceLength > 27 {
			tempMaxExp := uint(27)
			if tempMaxExp > maxExp { // Keep tracker's max if more restrictive
				tempMaxExp = maxExp
			}
			maxExp = tempMaxExp
		} else {
			tempMaxExp := *maxPieceLength
			if tempMaxExp > maxExp { // Keep tracker's max if more restrictive
				tempMaxExp = maxExp
			}
			maxExp = tempMaxExp
		}
	}

	// ensure minimum of 1 byte for calculation
	size := max(totalSize, 1)

	var exp uint
	switch {
	case size <= 64<<20: // 0 to 64 MB: 32 KiB pieces (2^15)
		exp = 15
	case size <= 128<<20: // 64-128 MB: 64 KiB pieces (2^16)
		exp = 16
	case size <= 256<<20: // 128-256 MB: 128 KiB pieces (2^17)
		exp = 17
	case size <= 512<<20: // 256-512 MB: 256 KiB pieces (2^18)
		exp = 18
	case size <= 1024<<20: // 512 MB-1 GB: 512 KiB pieces (2^19)
		exp = 19
	case size <= 2048<<20: // 1-2 GB: 1 MiB pieces (2^20)
		exp = 20
	case size <= 4096<<20: // 2-4 GB: 2 MiB pieces (2^21)
		exp = 21
	case size <= 8192<<20: // 4-8 GB: 4 MiB pieces (2^22)
		exp = 22
	case size <= 16384<<20: // 8-16 GB: 8 MiB pieces (2^23)
		exp = 23
	case size <= 32768<<20: // 16-32 GB: 16 MiB pieces (2^24)
		exp = 24
	case size <= 65536<<20: // 32-64 GB: 32 MiB pieces (2^25)
		exp = 25
	case size <= 131072<<20: // 64-128 GB: 64 MiB pieces (2^26)
		exp = 26
	default: // above 128 GB: 128 MiB pieces (2^27)
		exp = 27
	}

	// If no manual piece length was specified, cap at 2^24
	if maxPieceLength == nil && exp > 24 {
		exp = 24
	}

	// Ensure we stay within bounds
	if exp > maxExp {
		exp = maxExp
	}

	return exp
}

// FormatPieceSize returns a human readable piece size,
// using KiB for sizes < 1024 KiB and MiB for larger sizes
func FormatPieceSize(exp uint) string {
	size := uint64(1) << (exp - 10) // convert to KiB
	if size >= 1024 {
		return fmt.Sprintf("%d MiB", size/1024)
	}
	return fmt.Sprintf("%d KiB", size)
}
