//go:build windows

package torrent

// checkReplaceable has nothing to do on Windows, because MoveFileEx refuses a read-only file.
func checkReplaceable(string) error {
	return nil
}
