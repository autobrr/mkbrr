package torrent

import (
	"sync"
	"sync/atomic"
)

// BufferArena provides a memory arena for reusing buffers across operations.
// It reduces GC pressure by maintaining a pool of pre-allocated buffers.
// Based on kylesanderson/go-bencode/pkg/memarena.
type BufferArena struct {
	// Byte buffers for string/bytes data
	byteBufs [][]byte
	// Generic typed buffers - using interface{} for type erasure
	bufs     []any
	mu       sync.Mutex
	allocated int64
	reused    int64
}

// NewBufferArena creates a new buffer arena with pre-allocated capacity.
func NewBufferArena(byteBufCap, bufCap int) *BufferArena {
	return &BufferArena{
		byteBufs: make([][]byte, 0, byteBufCap),
		bufs:     make([]any, 0, bufCap),
	}
}

// GetBytes returns a byte slice with at least the given capacity.
// The slice is zeroed and ready for use.
func (a *BufferArena) GetBytes(capacity int) []byte {
	if capacity <= 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Try to reuse an existing buffer
	for i := range a.byteBufs {
		if cap(a.byteBufs[i]) >= capacity {
			buf := a.byteBufs[i][:capacity]
			// Remove from pool
			a.byteBufs = append(a.byteBufs[:i], a.byteBufs[i+1:]...)
			atomic.AddInt64(&a.reused, 1)
			return buf
		}
	}
	// Allocate new
	atomic.AddInt64(&a.allocated, 1)
	return make([]byte, capacity)
}

// PutBytes returns a byte slice to the arena for reuse.
// The slice's capacity is preserved, length is reset to 0.
func (a *BufferArena) PutBytes(buf []byte) {
	if buf == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.byteBufs = append(a.byteBufs, buf[:0])
}

// GetSlice returns a typed slice with at least the given capacity and length 0.
// Uses type parameter for type safety.
func GetSlice[T any](a *BufferArena, capacity int) []T {
	// For capacity 0, return an empty slice (not nil) so callers can distinguish
	// between "no slice allocated" and "empty slice allocated"
	if capacity == 0 {
		return make([]T, 0)
	}
	if capacity < 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Try to find a compatible buffer in the pool
	for i := range a.bufs {
		if s, ok := a.bufs[i].([]T); ok && cap(s) >= capacity {
			result := s[:0] // len=0, cap=cap(s) >= capacity
			a.bufs = append(a.bufs[:i], a.bufs[i+1:]...)
			atomic.AddInt64(&a.reused, 1)
			return result
		}
	}
	// Allocate new with requested capacity, len=0
	atomic.AddInt64(&a.allocated, 1)
	return make([]T, 0, capacity)
}

// PutSlice returns a typed slice to the arena for reuse.
func PutSlice[T any](a *BufferArena, s []T) {
	if s == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bufs = append(a.bufs, s[:0])
	atomic.AddInt64(&a.reused, 1)
}

// Stats returns allocation statistics.
func (a *BufferArena) Stats() (allocated, reused int64) {
	return atomic.LoadInt64(&a.allocated), atomic.LoadInt64(&a.reused)
}

// PieceHashArena provides memory arena for piece hash storage.
// It pre-allocates all piece hashes in a single contiguous block.
type PieceHashArena struct {
	storage []byte
	pieces  [][]byte
}

// NewPieceHashArena creates a new piece hash arena for the given number of pieces.
func NewPieceHashArena(numPieces int) *PieceHashArena {
	storage := make([]byte, numPieces*20) // SHA1 size is 20 bytes
	pieces := make([][]byte, numPieces)
	for i := range pieces {
		start := i * 20
		pieces[i] = storage[start : start+20 : start+20]
	}
	return &PieceHashArena{
		storage: storage,
		pieces:  pieces,
	}
}

// Get returns the slice for the given piece index.
func (a *PieceHashArena) Get(index int) []byte {
	return a.pieces[index]
}

// Storage returns the underlying storage slice.
func (a *PieceHashArena) Storage() []byte {
	return a.storage
}
