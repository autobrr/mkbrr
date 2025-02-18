//go:build !amd64
// +build !amd64

package sha1

import "crypto/sha1"

// blockSIMD is a fallback implementation for non-AMD64 architectures
// that uses the standard crypto/sha1 package
func blockSIMD(h *[5]uint32, p []byte) {
	// Create a new SHA1 hash
	hash := sha1.New()

	// Write the block
	hash.Write(p)

	// Get the current state
	state := hash.Sum(nil)

	// Convert to uint32 and store in h
	for i := 0; i < 5; i++ {
		h[i] = uint32(state[i*4])<<24 | uint32(state[i*4+1])<<16 | uint32(state[i*4+2])<<8 | uint32(state[i*4+3])
	}
}
