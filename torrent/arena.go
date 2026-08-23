package torrent

import (
	"sync"
	"sync/atomic"
)

// shardCount is the number of shards for the sharded arena.
// Using a power of 2 for efficient modulo via bitmask.
const shardCount = 16

// BufferArena provides a memory arena for reusing buffers across operations.
// It reduces GC pressure by maintaining a pool of pre-allocated buffers.
// Uses sharded locking to reduce contention under high concurrency.
// Based on kylesanderson/go-bencode/pkg/memarena.
type BufferArena struct {
	shards    [shardCount]arenaShard
	allocated int64
	reused    int64
}

type arenaShard struct {
	byteBufs [][]byte
	bufs     []any
	mu       sync.Mutex
}

// NewBufferArena creates a new buffer arena with pre-allocated capacity.
func NewBufferArena(byteBufCap, bufCap int) *BufferArena {
	a := &BufferArena{}
	perShardByteCap := (byteBufCap + shardCount - 1) / shardCount
	perShardBufCap := (bufCap + shardCount - 1) / shardCount
	for i := range a.shards {
		a.shards[i].byteBufs = make([][]byte, 0, perShardByteCap)
		a.shards[i].bufs = make([]any, 0, perShardBufCap)
	}
	return a
}

func (a *BufferArena) shard(capacity int) *arenaShard {
	// Use capacity as a simple hash to distribute across shards
	// This helps distribute different buffer sizes across shards
	return &a.shards[capacity&(shardCount-1)]
}

// GetBytes returns a byte slice with at least the given capacity.
// The slice is zeroed and ready for use.
func (a *BufferArena) GetBytes(capacity int) []byte {
	if capacity <= 0 {
		return nil
	}
	shard := a.shard(capacity)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	// Try to reuse an existing buffer
	for i := range shard.byteBufs {
		if cap(shard.byteBufs[i]) >= capacity {
			buf := shard.byteBufs[i][:capacity]
			// Remove from pool
			shard.byteBufs = append(shard.byteBufs[:i], shard.byteBufs[i+1:]...)
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
	shard := a.shard(cap(buf))
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.byteBufs = append(shard.byteBufs, buf[:0])
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
	shard := a.shard(capacity)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	
	// Try to find a compatible buffer in the pool
	for i := range shard.bufs {
		if s, ok := shard.bufs[i].([]T); ok && cap(s) >= capacity {
			result := s[:0] // len=0, cap=cap(s) >= capacity
			shard.bufs = append(shard.bufs[:i], shard.bufs[i+1:]...)
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
	shard := a.shard(cap(s))
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.bufs = append(shard.bufs, s[:0])
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
