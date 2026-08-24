//go:build windows

package torrent

import "golang.org/x/sys/windows"

// publishTorrentFile installs tempPath at outputPath, optionally replacing an existing entry.
func publishTorrentFile(tempPath, outputPath string, replace bool) error {
	tempPathPointer, err := windows.UTF16PtrFromString(tempPath)
	if err != nil {
		return err
	}
	outputPathPointer, err := windows.UTF16PtrFromString(outputPath)
	if err != nil {
		return err
	}

	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if replace {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	return windows.MoveFileEx(tempPathPointer, outputPathPointer, flags)
}
