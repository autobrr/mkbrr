//go:build !windows

package torrent

import "os"

// publishTorrentFile installs tempPath at outputPath, optionally replacing an existing entry.
func publishTorrentFile(tempPath, outputPath string, replace bool) error {
	if replace {
		return os.Rename(tempPath, outputPath)
	}

	// ponytail: atomic no-replace publication requires hard-link support; add a native primitive if unsupported filesystems become a user requirement.
	if err := os.Link(tempPath, outputPath); err != nil {
		return err
	}
	_ = os.Remove(tempPath)
	return nil
}
