package torrent

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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

// normalizePattern converts a pattern to doublestar format for consistent matching.
// Simple patterns without path separators (like "*.nfo") are prefixed with "**/"
// to maintain backward compatibility and match files at any depth.
func normalizePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}

	// Normalize backslashes to forward slashes first
	pattern = strings.ReplaceAll(pattern, "\\", "/")

	// If pattern doesn't contain path separators and doesn't start with **,
	// it's a simple filename pattern - wrap it to match at any depth
	if !strings.Contains(pattern, "/") && !strings.HasPrefix(pattern, "**") {
		return "**/" + pattern
	}

	return pattern
}

// splitPatterns splits a comma-separated pattern string into individual patterns,
// respecting brace expressions like {a,b,c} which should not be split.
func splitPatterns(patternGroup string) []string {
	var patterns []string
	var current strings.Builder
	braceDepth := 0

	for _, r := range patternGroup {
		switch r {
		case '{':
			braceDepth++
			current.WriteRune(r)
		case '}':
			braceDepth--
			current.WriteRune(r)
		case ',':
			if braceDepth > 0 {
				// Inside braces, keep the comma
				current.WriteRune(r)
			} else {
				// Outside braces, this is a pattern separator
				if s := strings.TrimSpace(current.String()); s != "" {
					patterns = append(patterns, s)
				}
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	// Don't forget the last pattern
	if s := strings.TrimSpace(current.String()); s != "" {
		patterns = append(patterns, s)
	}

	return patterns
}

// matchPattern matches a pattern against a path using doublestar.
// It handles case-insensitivity and proper directory matching.
func matchPattern(pattern, relPath string, isDir bool) (bool, error) {
	if pattern == "" || relPath == "" {
		return false, nil
	}

	pattern = normalizePattern(pattern)
	lowerPattern := strings.ToLower(pattern)
	lowerPath := strings.ToLower(filepath.ToSlash(relPath))

	// Try matching the path directly
	match, err := doublestar.Match(lowerPattern, lowerPath)
	if err != nil {
		return false, err
	}
	if match {
		return true, nil
	}

	// For directories, also try matching with trailing slash
	// This allows patterns like "**/extras/**" to match directory "extras"
	if isDir {
		match, err = doublestar.Match(lowerPattern, lowerPath+"/")
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}

		// For patterns like "**/dirname/**", check if the directory is in the path
		// This allows early directory skipping for recursive exclude patterns
		if strings.HasPrefix(lowerPattern, "**/") && strings.HasSuffix(lowerPattern, "/**") {
			// Extract the middle part: "**/extras/**" -> "extras"
			middle := strings.TrimPrefix(lowerPattern, "**/")
			middle = strings.TrimSuffix(middle, "/**")
			// Only match if middle is a simple literal (no wildcards)
			if !strings.ContainsAny(middle, "*?[{") {
				pathParts := strings.Split(lowerPath, "/")
				if slices.Contains(pathParts, middle) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// shouldIgnoreEntry checks if a file or directory should be ignored based on
// predefined patterns, user-defined include patterns, and user-defined exclude patterns.
// It uses doublestar for full glob support including ** recursive matching.
//
// Parameters:
//   - relPath: relative path from the torrent root (using forward slashes)
//   - isDir: true if the entry is a directory
//   - excludePatterns: patterns to exclude (glob syntax)
//   - includePatterns: patterns to include (glob syntax, acts as whitelist)
//
// Logic:
//  1. Check hardcoded ignored directory names (always ignored).
//  2. Check built-in ignored file patterns (always ignored).
//  3. If include patterns are provided:
//     - For directories: always traverse (return false) to find matching files inside.
//     - For files: must match at least one include pattern, otherwise ignored.
//  4. Check exclude patterns: if matched, ignore the entry.
//  5. If none of the above, keep the entry.
func shouldIgnoreEntry(relPath string, isDir bool, excludePatterns []string, includePatterns []string) (bool, error) {
	if relPath == "" || relPath == "." {
		return false, nil
	}

	// Normalize path to forward slashes
	relPath = filepath.ToSlash(relPath)
	lowerRelPath := strings.ToLower(relPath)

	// 1. Check hardcoded ignored directory names (safety net)
	segments := strings.Split(lowerRelPath, "/")
	for _, segment := range segments {
		if slices.Contains(ignoredDirNames, segment) {
			return true, nil
		}
	}

	// 2. Check built-in ignored patterns for files (always ignored)
	if !isDir {
		for _, pattern := range ignoredPatterns {
			if strings.HasSuffix(lowerRelPath, pattern) {
				return true, nil
			}
		}
	}

	// 3. Check include patterns if provided
	if len(includePatterns) > 0 {
		// For directories: always traverse to find matching files inside
		if isDir {
			return false, nil
		}

		// For files: must match at least one include pattern
		matchesInclude := false
		for _, patternGroup := range includePatterns {
			for _, pattern := range splitPatterns(patternGroup) {
				if pattern == "" {
					continue
				}
				match, err := matchPattern(pattern, relPath, false)
				if err != nil {
					return false, err
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

		if !matchesInclude {
			return true, nil // Ignore file because no include pattern matched
		}

		return false, nil // Keep file because include patterns are a whitelist
	}

	// 4. Check exclude patterns
	if len(excludePatterns) > 0 {
		for _, patternGroup := range excludePatterns {
			for _, pattern := range splitPatterns(patternGroup) {
				if pattern == "" {
					continue
				}
				match, err := matchPattern(pattern, relPath, isDir)
				if err != nil {
					return false, err
				}
				if match {
					return true, nil // Ignore because it matches exclude pattern
				}
			}
		}
	}

	// 5. Keep the entry (don't ignore)
	return false, nil
}

// shouldIgnoreFile checks if a file should be ignored based on predefined patterns,
// user-defined include patterns, and user-defined exclude patterns (glob matching).
// This is a wrapper around shouldIgnoreEntry for backward compatibility.
//
// Deprecated: Use shouldIgnoreEntry directly for new code.
func shouldIgnoreFile(path string, excludePatterns []string, includePatterns []string) (bool, error) {
	// For backward compatibility, extract just the filename and match against it
	// This maintains the old behavior when called with absolute paths
	filename := filepath.Base(path)
	return shouldIgnoreEntry(filename, false, excludePatterns, includePatterns)
}

// shouldIgnoreDir checks if any directory segment in the path should be ignored.
// This checks against the hardcoded ignoredDirNames list.
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
