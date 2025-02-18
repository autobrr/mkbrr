//go:build !amd64
// +build !amd64

package sha1

import (
	"crypto/sha1"
	"hash"
)

// newSHA1SIMD is not available on non-amd64 platforms
func newSHA1SIMD() hash.Hash {
	return sha1.New()
}
