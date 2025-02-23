//go:build !isal || (!linux && !amd64)

package torrent

import (
	"crypto/sha1"
	"hash"
)

// newSHA1 returns a new hash.Hash computing the SHA1 checksum using standard library.
func newSHA1() hash.Hash {
	return sha1.New()
}
