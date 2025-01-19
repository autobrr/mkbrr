package torrent

import "strings"

// file patterns to ignore (case insensitive)
var ignoredPatterns = []string{
	".torrent",
	".ds_store",
	"thumbs.db",
	"desktop.ini",
}

// shouldIgnoreFile checks if a file should be ignored based on predefined patterns
func shouldIgnoreFile(path string) bool {
	lowerPath := strings.ToLower(path)
	for _, pattern := range ignoredPatterns {
		if strings.HasSuffix(lowerPath, pattern) {
			return true
		}
	}
	return false
}
