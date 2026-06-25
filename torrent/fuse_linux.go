//go:build linux

package torrent

import "golang.org/x/sys/unix"

// fuseSuperMagic is the f_type reported by statfs(2) for FUSE mounts
// (mergerfs, sshfs, rclone mount, etc.). See <linux/magic.h> FUSE_SUPER_MAGIC.
const fuseSuperMagic = 0x65735546

// isFUSEPath reports whether path resides on a FUSE filesystem. mmap reads on
// FUSE fault page-by-page through the userspace daemon rather than benefiting
// from read() readahead, so the hasher prefers the read() path there.
func isFUSEPath(path string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false // can't tell — assume not FUSE and let the size check handle issues
	}
	return st.Type == fuseSuperMagic
}
