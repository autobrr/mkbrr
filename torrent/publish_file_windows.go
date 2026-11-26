//go:build windows

package torrent

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

const (
	publishRetryBudget   = 2 * time.Second
	publishRetryMaxSleep = 50 * time.Millisecond
)

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
	deadline := time.Now().Add(publishRetryBudget)
	for sleep := time.Millisecond; ; sleep = min(2*sleep, publishRetryMaxSleep) {
		err = windows.MoveFileEx(tempPathPointer, outputPathPointer, flags)
		if !isTransientPublishError(err) || sleep > time.Until(deadline) {
			return err
		}
		time.Sleep(sleep)
	}
}

func isTransientPublishError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
