package sha1

import (
	"crypto/sha1"
	"hash"
	"runtime"
	"sync"

	"github.com/klauspost/cpuid/v2"
)

// hasherPool is a pool of SHA1 hashers to reduce allocations
var hasherPool = sync.Pool{
	New: func() interface{} {
		if useHWHash() {
			return newSHA1SIMD()
		}
		return sha1.New()
	},
}

// New returns a new hash.Hash computing the SHA1 checksum.
// It will use hardware acceleration if available.
func New() hash.Hash {
	h := hasherPool.Get().(hash.Hash)
	h.Reset()
	runtime.SetFinalizer(h, func(h hash.Hash) {
		hasherPool.Put(h)
	})
	return h
}

// useHWHash returns true if hardware SHA1 acceleration should be used
func useHWHash() bool {
	// Only use hardware acceleration on amd64 and arm64
	switch runtime.GOARCH {
	case "amd64":
		return cpuid.CPU.Has(cpuid.SHA)
	case "arm64":
		return cpuid.CPU.Has(cpuid.SHA1)
	default:
		return false
	}
}
