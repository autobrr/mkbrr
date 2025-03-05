package utils

import (
	"fmt"
	"strings"
)

// FormatPieceSize returns a human readable piece size,
// using KiB for sizes < 1024 KiB and MiB for larger sizes
func FormatPieceSize(exp uint) string {
	size := uint64(1) << (exp - 10) // convert to KiB
	if size >= 1024 {
		return fmt.Sprintf("%d MiB", size/1024)
	}
	return fmt.Sprintf("%d KiB", size)
}

// SanitizeFilename removes characters that are invalid in filenames
func SanitizeFilename(input string) string {
	// replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(input)
}

func GetDomainPrefix(trackerURL string) string {
	if trackerURL == "" {
		return "modified"
	}

	cleanURL := strings.TrimSpace(trackerURL)

	domain := cleanURL

	if strings.Contains(domain, "://") {
		parts := strings.SplitN(domain, "://", 2)
		if len(parts) == 2 {
			domain = parts[1]
		}
	}

	if strings.Contains(domain, "/") {
		domain = strings.SplitN(domain, "/", 2)[0]
	}

	if strings.Contains(domain, ":") {
		domain = strings.SplitN(domain, ":", 2)[0]
	}

	domain = strings.TrimPrefix(domain, "www.")

	if domain != "" {
		parts := strings.Split(domain, ".")

		if len(parts) > 1 {
			// take only the domain name without TLD
			// for example, from "tracker.example.com", get "example"
			if len(parts) > 2 {
				// for subdomains, use the second-to-last part
				domain = parts[len(parts)-2]
			} else {
				// for simple domains like example.com, use the first part
				domain = parts[0]
			}
		}

		return SanitizeFilename(domain)
	}

	return "modified"
}
