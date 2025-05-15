package guiapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getDefaultSaveDirectory attempts to find the best default directory for saving torrent files.
// It prioritizes Desktop, then Home, and returns an error if Home cannot be determined.
func getDefaultSaveDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	desktopPath := filepath.Join(homeDir, "Desktop")
	info, err := os.Stat(desktopPath)
	if err == nil && info.IsDir() {
		return desktopPath, nil
	}

	return homeDir, nil
}

func parseExcludePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseIncludePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
