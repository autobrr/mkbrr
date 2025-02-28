//go:build linux && amd64 && isal

package torrent

/*
#cgo CFLAGS: -I/usr/local/include -I/usr/include
#cgo LDFLAGS: -L/usr/local/lib -L/usr/lib -lisal_crypto

#include <isa-l_crypto.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

// Use the multi-hash SHA1 implementation
void sha1_init_wrapper(struct isal_mh_sha1_ctx *ctx) {
    isal_mh_sha1_init(ctx);
}

void sha1_update_wrapper(struct isal_mh_sha1_ctx *ctx, const void *data, size_t len) {
    isal_mh_sha1_update(ctx, data, (uint32_t)len);
}

void sha1_final_wrapper(struct isal_mh_sha1_ctx *ctx, uint8_t *out) {
    uint32_t digest[5];  // SHA1 produces a 20-byte (5 uint32_t) hash
    isal_mh_sha1_finalize(ctx, digest);
    memcpy(out, digest, 20);
}
*/
import "C"
import (
	"hash"
	"runtime"
	"unsafe"
)

const (
	// Size of SHA1 checksum in bytes
	sha1Size = 20
	// BlockSize is the block size of SHA1 in bytes
	sha1BlockSize = 64
)

// isalSHA1 is a wrapper around ISA-L's SHA1 implementation
type isalSHA1 struct {
	ctx C.struct_isal_mh_sha1_ctx
	// used as a buffer for final output
	sum [sha1Size]byte
}

// newISALSHA1 returns a new hash.Hash computing SHA1 using ISA-L
func newISALSHA1() hash.Hash {
	h := &isalSHA1{}
	runtime.SetFinalizer(h, func(h *isalSHA1) {
		h.Reset()
	})
	h.Reset()
	return h
}

// Reset resets the hash state
func (h *isalSHA1) Reset() {
	C.sha1_init_wrapper(&h.ctx)
}

// Size returns the size of a SHA1 checksum in bytes
func (h *isalSHA1) Size() int {
	return sha1Size
}

// BlockSize returns the block size of SHA1 in bytes
func (h *isalSHA1) BlockSize() int {
	return sha1BlockSize
}

// Write adds more data to the hash state
func (h *isalSHA1) Write(p []byte) (int, error) {
	if len(p) > 0 {
		C.sha1_update_wrapper(&h.ctx, unsafe.Pointer(&p[0]), C.size_t(len(p)))
	}
	return len(p), nil
}

// Sum appends the SHA1 checksum to b
func (h *isalSHA1) Sum(b []byte) []byte {
	// Copy context to avoid modifying the current state
	ctxCopy := h.ctx

	// Get final hash
	C.sha1_final_wrapper(&ctxCopy, (*C.uint8_t)(&h.sum[0]))

	if b == nil {
		return h.sum[:]
	}
	return append(b, h.sum[:]...)
}
