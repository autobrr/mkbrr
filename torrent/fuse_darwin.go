//go:build darwin

package torrent

import (
	"strings"

	"golang.org/x/sys/unix"
)

// isFUSEPath reports whether path resides on a FUSE filesystem (macFUSE/osxfuse).
// mmap reads on FUSE fault page-by-page through the userspace daemon rather than
// benefiting from read() readahead, so the hasher prefers the read() path there.
func isFUSEPath(path string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false
	}
	name := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return strings.Contains(strings.ToLower(string(name)), "fuse")
}
