//go:build !windows

package torrent

import (
	"os"

	"golang.org/x/sys/unix"
)

// checkReplaceable refuses a file that the user cannot write, as the earlier in-place write did.
func checkReplaceable(path string) error {
	if err := unix.Access(path, unix.W_OK); err != nil {
		return &os.PathError{Op: "replace", Path: path, Err: err}
	}
	return nil
}
