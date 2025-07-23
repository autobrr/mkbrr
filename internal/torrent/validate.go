package torrent

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/trackers"
)

// ValidationStatus represents the outcome of a single rule check.
type ValidationStatus string

const (
	ValidationPass ValidationStatus = "PASS"
	ValidationFail ValidationStatus = "FAIL"
	ValidationWarn ValidationStatus = "WARN"
	ValidationInfo ValidationStatus = "INFO"
	ValidationSkip ValidationStatus = "SKIP"
)

// ValidationResult holds the outcome of a single validation rule check.
type ValidationResult struct {
	Rule    string           `json:"rule"`
	Status  ValidationStatus `json:"status"`
	Message string           `json:"message"`
}

// ValidateAgainstTrackerRules checks a torrent's metadata against known rules for a specific tracker.
// It returns a slice of ValidationResult, detailing the outcome of each check.
func ValidateAgainstTrackerRules(mi *metainfo.MetaInfo, info *metainfo.Info, trackerURL string, rawTorrentBytes []byte) ([]ValidationResult, error) {
	results := []ValidationResult{}

	// 1. Get Tracker Config
	trackerConfig := trackers.FindTrackerConfig(trackerURL)
	displayURL := trackerURL
	parsedURL, err := url.Parse(trackerURL)
	if err == nil {
		displayURL = parsedURL.Scheme + "://" + parsedURL.Host + "/..."
	} else {
		displayURL = "the specified tracker"
	}

	announceMatch := false
	if mi.Announce == trackerURL {
		announceMatch = true
	} else {
		for _, tier := range mi.AnnounceList {
			for _, announce := range tier {
				if announce == trackerURL {
					announceMatch = true
					break
				}
				if trackerConfig != nil {
					for _, baseURL := range trackerConfig.URLs {
						if strings.Contains(announce, baseURL) {
							announceMatch = true
							break
						}
					}
				}
				if announceMatch {
					break
				}
			}
			if announceMatch {
				break
			}
		}
	}
	if announceMatch {
		results = append(results, ValidationResult{
			Rule:    "Announce URL",
			Status:  ValidationPass,
			Message: "Torrent contains an announce URL matching the specified tracker/preset.",
		})
	} else {
		results = append(results, ValidationResult{
			Rule:    "Announce URL",
			Status:  ValidationFail,
			Message: fmt.Sprintf("Torrent does not contain an announce URL matching %s or its known aliases.", displayURL),
		})
	}

	// If no specific config, add the skip message and return (announce check result is already included)
	if trackerConfig == nil {
		results = append(results, ValidationResult{
			Rule:    "Tracker Recognition",
			Status:  ValidationSkip,
			Message: fmt.Sprintf("No specific rules found for tracker URL containing '%s'. Cannot perform detailed validation.", displayURL),
		})
		return results, nil
	}

	isPrivateRequired := true
	if info.Private == nil || !*info.Private {
		if isPrivateRequired {
			results = append(results, ValidationResult{
				Rule:    "Private Flag",
				Status:  ValidationFail,
				Message: "Torrent is not marked as private, but the tracker likely requires it.",
			})
		} else {
			results = append(results, ValidationResult{
				Rule:    "Private Flag",
				Status:  ValidationWarn,
				Message: "Torrent is not marked as private.",
			})
		}
	} else {
		results = append(results, ValidationResult{
			Rule:    "Private Flag",
			Status:  ValidationPass,
			Message: "Torrent is marked as private.",
		})
	}

	if maxExp, ok := trackers.GetTrackerMaxPieceLength(trackerURL); ok {
		maxPieceLenBytes := int64(1) << maxExp
		currentExp := uint(0)
		for p := info.PieceLength; p > 1; p >>= 1 {
			currentExp++
		}

		if info.PieceLength > maxPieceLenBytes {
			results = append(results, ValidationResult{
				Rule:    "Piece Size Limit",
				Status:  ValidationFail,
				Message: fmt.Sprintf("Piece size %s exceeds tracker limit of %s.", FormatPieceSize(currentExp), FormatPieceSize(maxExp)),
			})
		} else {
			results = append(results, ValidationResult{
				Rule:    "Piece Size Limit",
				Status:  ValidationPass,
				Message: fmt.Sprintf("Piece size %s is within tracker limit of %s.", FormatPieceSize(currentExp), FormatPieceSize(maxExp)),
			})
		}
	} else {
		results = append(results, ValidationResult{
			Rule:    "Piece Size Limit",
			Status:  ValidationInfo,
			Message: fmt.Sprintf("No specific piece size limit known for this tracker. Current size: %s.", FormatPieceSize(uint(info.PieceLength))),
		})
	}

	if maxTorrentSize, ok := trackers.GetTrackerMaxTorrentSize(trackerURL); ok {
		if uint64(len(rawTorrentBytes)) > maxTorrentSize {
			results = append(results, ValidationResult{
				Rule:    "Torrent File Size",
				Status:  ValidationFail,
				Message: fmt.Sprintf("Torrent file size %s exceeds tracker limit of %s.", FormatBytes(int64(len(rawTorrentBytes))), FormatBytes(int64(maxTorrentSize))),
			})
		} else {
			results = append(results, ValidationResult{
				Rule:    "Torrent File Size",
				Status:  ValidationPass,
				Message: fmt.Sprintf("Torrent file size %s is within tracker limit of %s.", FormatBytes(int64(len(rawTorrentBytes))), FormatBytes(int64(maxTorrentSize))),
			})
		}
	} else {
		results = append(results, ValidationResult{
			Rule:    "Torrent File Size",
			Status:  ValidationInfo,
			Message: fmt.Sprintf("No specific torrent file size limit known for this tracker. Current size: %s.", FormatBytes(int64(len(rawTorrentBytes)))),
		})
	}

	// Check Source Tag (Placeholder - assumes TrackerConfig might get a 'RequiresSourceTag' field)
	// if trackerConfig.RequiresSourceTag { // Hypothetical field
	// 	if info.Source == "" {
	// 		results = append(results, ValidationResult{
	// 			Rule:    "Source Tag",
	// 			Status:  ValidationFail,
	// 			Message: "Source tag is missing, but the tracker requires it.",
	// 		})
	// 	} else {
	// 		results = append(results, ValidationResult{
	// 			Rule:    "Source Tag",
	// 			Status:  ValidationPass,
	// 			Message: fmt.Sprintf("Source tag '%s' is present.", info.Source),
	// 		})
	// 	}
	// } else {
	//if info.Source != "" {
	//	results = append(results, ValidationResult{
	//		Rule:    "Source Tag",
	//		Status:  ValidationInfo,
	//		Message: fmt.Sprintf("Source tag is present: '%s'.", info.Source),
	//	})
	// } else {
	// 	results = append(results, ValidationResult{
	// 		Rule:    "Source Tag",
	//		Status:  ValidationInfo,
	//		Message: "Source tag is not present (optional for many trackers).",
	//	})
	// The announce check logic previously here has been moved up.

	if recommendedExp, ok := trackers.GetTrackerPieceSizeExp(trackerURL, uint64(info.TotalLength())); ok {
		currentExp := uint(0)
		for p := info.PieceLength; p > 1; p >>= 1 {
			currentExp++
		}

		if currentExp != recommendedExp {
			results = append(results, ValidationResult{
				Rule:    "Recommended Piece Size",
				Status:  ValidationWarn,
				Message: fmt.Sprintf("Current piece size (%s) differs from tracker recommendation (%s) for this content size.", FormatPieceSize(currentExp), FormatPieceSize(recommendedExp)),
			})
		} else {
			results = append(results, ValidationResult{
				Rule:    "Recommended Piece Size",
				Status:  ValidationPass,
				Message: fmt.Sprintf("Current piece size (%s) matches tracker recommendation.", FormatPieceSize(currentExp)),
			})
		}
	} else {
		if !trackerConfig.UseDefaultRanges && len(trackerConfig.PieceSizeRanges) > 0 {
			results = append(results, ValidationResult{
				Rule:    "Recommended Piece Size",
				Status:  ValidationInfo,
				Message: "No specific recommendation for this content size, using default calculation or highest range.",
			})
		}
	}

	// TODO: Add checks for Disallowed Fields (requires TrackerConfig update & bencode inspection)

	return results, nil
}

// Helper function (already exists in create.go, maybe move to a common place?)
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function (already exists in create.go, maybe move to a common place?)
func FormatPieceSize(exp uint) string {
	size := uint64(1) << (exp - 10) // convert to KiB
	if size >= 1024 {
		return fmt.Sprintf("%d MiB", size/1024)
	}
	return fmt.Sprintf("%d KiB", size)
}
