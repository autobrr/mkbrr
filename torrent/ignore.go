package torrent

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// file patterns to ignore in source directory (case insensitive) - These are always ignored.
var ignoredPatterns = []string{
	".torrent",
	".ds_store",
	"thumbs.db",
	"desktop.ini",
	"zone.identifier", // https://superuser.com/questions/1692240/auto-generated-zone-identity-files-can-should-i-delete
}

// directories to ignore in source directory (case insensitive) - These are always ignored.
var ignoredDirNames = []string{
	"@eadir",
}

// shouldIgnoreFile checks if a file should be ignored based on predefined patterns,
// user-defined include patterns, and user-defined exclude patterns (glob matching).
// Logic:
// 1. Check built-in ignored patterns (always ignored).
// 2. If include patterns are provided:
//   - Check if the file matches any include pattern. If yes, KEEP the file (return false).
//   - If it does not match any include pattern, IGNORE the file (return true).
//
// 3. If NO include patterns are provided:
//   - Check if the file matches any exclude pattern. If yes, IGNORE the file (return true).
//
// 4. If none of the above conditions cause the file to be ignored, KEEP the file (return false).
func shouldIgnoreFile(path string, excludePatterns []string, includePatterns []string) (bool, error) {
	if shouldIgnoreDir(path) {
		return true, nil
	}

	// 1. Check built-in patterns (always ignored)
	lowerPath := strings.ToLower(path)
	for _, pattern := range ignoredPatterns {
		if strings.HasSuffix(lowerPath, pattern) {
			return true, nil
		}
	}

	filename := filepath.Base(path)
	lowerFilename := strings.ToLower(filename)

	// 2. Check include patterns if provided
	if len(includePatterns) > 0 {
		matchesInclude := false
		for _, patternGroup := range includePatterns {
			for _, pattern := range strings.Split(patternGroup, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}
				match, err := filepath.Match(strings.ToLower(pattern), lowerFilename)
				if err != nil {
					return false, fmt.Errorf("invalid include pattern %q: %w", pattern, err)
				}
				if match {
					matchesInclude = true
					break
				}
			}
			if matchesInclude {
				break
			}
		}

		if matchesInclude {
			return false, nil // Keep the file because it matches an include pattern
		} else {
			return true, nil // Ignore the file because include patterns were given, but none matched
		}
	}

	// 3. If NO include patterns were provided, check exclude patterns
	if len(excludePatterns) > 0 {
		for _, patternGroup := range excludePatterns {
			for _, pattern := range strings.Split(patternGroup, ",") {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}
				match, err := filepath.Match(strings.ToLower(pattern), lowerFilename)
				if err != nil {
					return false, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
				}
				if match {
					return true, nil // Ignore if it matches an exclude pattern (and no include patterns were specified)
				}
			}
		}
	}

	// 4. Keep the file if no ignore conditions were met
	return false, nil
}

// shouldIgnoreDir checks if any directory segment in the path should be ignored.
func shouldIgnoreDir(path string) bool {
	lowerPath := strings.ToLower(path)
	segments := strings.FieldsFunc(lowerPath, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	for _, segment := range segments {
		if slices.Contains(ignoredDirNames, segment) {
			return true
		}
	}

	return false
}
