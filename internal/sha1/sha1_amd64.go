//go:build amd64
// +build amd64

package sha1

import (
	"encoding/binary"
	"hash"
)

// blockSIMD is implemented in sha1block_amd64.s
//
//go:noescape
func blockSIMD(h *[5]uint32, p []byte)

// size of a SHA1 checksum in bytes
const Size = 20

// size of a SHA1 block in bytes
const BlockSize = 64

// sha1SIMD represents the partial evaluation of a SHA1 checksum
// using Intel SHA extensions
type sha1SIMD struct {
	h   [5]uint32 // current hash state
	x   [BlockSize]byte
	nx  int // index into x
	len uint64
}

// newSHA1SIMD returns a new hash.Hash computing the SHA1 checksum
// using Intel SHA extensions
func newSHA1SIMD() hash.Hash {
	h := new(sha1SIMD)
	h.Reset()
	return h
}

func (h *sha1SIMD) Reset() {
	// SHA1 initial hash values
	h.h[0] = 0x67452301
	h.h[1] = 0xEFCDAB89
	h.h[2] = 0x98BADCFE
	h.h[3] = 0x10325476
	h.h[4] = 0xC3D2E1F0
	h.nx = 0
	h.len = 0
}

func (h *sha1SIMD) Size() int { return Size }

func (h *sha1SIMD) BlockSize() int { return BlockSize }

func (h *sha1SIMD) Write(p []byte) (nn int, err error) {
	nn = len(p)
	h.len += uint64(nn)
	if h.nx > 0 {
		n := copy(h.x[h.nx:], p)
		h.nx += n
		if h.nx == BlockSize {
			blockSIMD(&h.h, h.x[:])
			h.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= BlockSize {
		n := len(p) &^ (BlockSize - 1)
		blockSIMD(&h.h, p[:n])
		p = p[n:]
	}
	if len(p) > 0 {
		h.nx = copy(h.x[:], p)
	}
	return
}

func (h *sha1SIMD) Sum(in []byte) []byte {
	// Make a copy of d so that caller can keep writing and summing.
	d0 := *h
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

func (h *sha1SIMD) checkSum() [Size]byte {
	len := h.len
	// Padding. Add a 1 bit and 0 bits until 56 bytes mod 64.
	var tmp [64]byte
	tmp[0] = 0x80
	if len%64 < 56 {
		h.Write(tmp[0 : 56-len%64])
	} else {
		h.Write(tmp[0 : 64+56-len%64])
	}

	// Length in bits.
	len <<= 3
	binary.BigEndian.PutUint64(tmp[:], len)
	h.Write(tmp[0:8])

	if h.nx != 0 {
		panic("h.nx != 0")
	}

	var digest [Size]byte
	for i, s := range h.h {
		binary.BigEndian.PutUint32(digest[i*4:], s)
	}
	return digest
}
