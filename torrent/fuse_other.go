//go:build !linux && !darwin

package torrent

// isFUSEPath is a no-op on platforms without a known FUSE statfs probe. On
// non-unix, mmap is unsupported anyway; on other unix (e.g. *BSD) mmap stays on
// and the read() fallback in feed handles any failure.
func isFUSEPath(path string) bool { return false }
