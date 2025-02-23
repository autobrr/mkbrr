//go:build isal && linux && amd64

package torrent

import (
	"hash"
	"sync"
)

var (
	logOnce sync.Once
	display = NewDisplay(NewFormatter(false))
)

// newSHA1 returns a new hash.Hash computing the SHA1 checksum.
// It will use ISA-L crypto library on supported platforms (x86_64 Linux)
// and fall back to standard library on other platforms.
func newSHA1() hash.Hash {
	logOnce.Do(func() {
		display.ShowMessage("using ISA-L SHA1 implementation")
	})
	return newISALSHA1()
}
