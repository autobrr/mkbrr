//go:build !isal || (!linux && !amd64)

package torrent

import (
	"crypto/sha1"
	"hash"
	"sync"
)

var (
	logOnce sync.Once
	display = NewDisplay(NewFormatter(false))
)

// newSHA1 returns a new hash.Hash computing the SHA1 checksum using standard library.
func newSHA1() hash.Hash {
	logOnce.Do(func() {
		display.ShowMessage("using standard library SHA1 implementation")
	})
	return sha1.New()
}
