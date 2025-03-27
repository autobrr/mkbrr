package torrent

import (
	"path/filepath"
	"strings"
)

// file patterns to ignore in source directory (case insensitive)
var ignoredPatterns = []string{
	".torrent",
	".ds_store",
	"thumbs.db",
	"desktop.ini",
}

// shouldIgnoreFile checks if a file should be ignored based on predefined patterns
// and user-defined exclude patterns (glob matching).
func shouldIgnoreFile(path string, excludePatterns []string) bool {
	// first check built-in patterns (exact suffix match, case insensitive)
	lowerPath := strings.ToLower(path)
	for _, pattern := range ignoredPatterns {
		if strings.HasSuffix(lowerPath, pattern) {
			return true
		}
	}

	// then check user-defined exclude patterns (glob matching on filename, case insensitive)
	if len(excludePatterns) > 0 {
		filename := filepath.Base(path)
		lowerFilename := strings.ToLower(filename)

		for _, patternGroup := range excludePatterns {
			for _, pattern := range strings.Split(patternGroup, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}

				match, err := filepath.Match(strings.ToLower(pattern), lowerFilename)
				// we ignore the error from filepath.Match as malformed patterns simply won't match
				if err == nil && match {
					return true
				}
			}
		}
	}

	return false
}
