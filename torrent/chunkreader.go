package torrent

import (
	"io"
	"os"
	"strconv"
	"sync"
)

// defaultMmapWindow is the size of a single mmap window. The worker maps at
// most this many bytes at a time and unmaps each window before advancing, so
// peak resident set from mapped file pages stays bounded at roughly
// (workers * defaultMmapWindow) regardless of file size — important on
// memory-constrained containers where mapping a whole multi-GiB file would
// otherwise inflate the process RSS by the file's size.
const defaultMmapWindow = 16 << 20 // 16 MiB

// minMmapFileSize is the smallest file for which mmap is worthwhile. For tiny
// inputs the map/unmap syscalls cost more than a couple of read()s, so the
// buffered path is used instead.
const minMmapFileSize = 1 << 20 // 1 MiB

// chunkReader feeds contiguous byte ranges of already-open files into a hash.
//
// Two implementations exist:
//   - bufferedChunkReader: a read()-based reader using a pooled buffer. This is
//     the portable default everywhere and the fallback when mmap is
//     unavailable or disabled.
//   - mmapChunkReader (unix only): maps windows of the file and feeds the
//     mapped pages straight into the hash, avoiding the per-read syscall and
//     the kernel->user copy. On FUSE/network filesystems (e.g. mergerfs) this
//     dramatically reduces sys CPU, which scales with read() syscall count.
//
// Both implementations are byte-for-byte equivalent; this is enforced by
// TestChunkReaderEquivalence and FuzzChunkReaderEquivalence.
type chunkReader interface {
	// feed writes fr.file's bytes [start, start+length) into h. It feeds
	// exactly length bytes or returns an error.
	feed(h io.Writer, fr *fileReader, start, length int64) error
	// release frees any resources held by the reader (e.g. returns a pooled
	// buffer).
	release()
}

// bufferedChunkReader reads through a pooled buffer using read()/seek. It
// preserves the historical hashing behavior exactly and is the fallback path.
// buf is a *[]byte so returning it to the pool does not allocate (SA6002).
type bufferedChunkReader struct {
	buf  *[]byte
	pool *sync.Pool
}

func (b *bufferedChunkReader) feed(h io.Writer, fr *fileReader, start, length int64) error {
	if fr.position != start {
		if _, err := fr.file.Seek(start, io.SeekStart); err != nil {
			return err
		}
		fr.position = start
	}

	buf := *b.buf
	remaining := length
	for remaining > 0 {
		n := min(int64(len(buf)), remaining)
		read, err := io.ReadFull(fr.file, buf[:n])
		if err != nil && err != io.ErrUnexpectedEOF {
			return err
		}
		if read == 0 {
			return io.ErrUnexpectedEOF
		}
		if _, werr := h.Write(buf[:read]); werr != nil {
			return werr
		}
		remaining -= int64(read)
		fr.position += int64(read)
	}
	return nil
}

func (b *bufferedChunkReader) release() {
	if b.pool != nil && b.buf != nil {
		b.pool.Put(b.buf)
		b.buf = nil
	}
}

// hashStrategy captures how a hasher reads file bytes.
type hashStrategy struct {
	useMmap    bool
	forced     bool // mmap explicitly requested via env; bypasses the FUSE auto-disable
	mmapWindow int64
}

// fuseDetect reports whether a path lives on a FUSE mount. Indirected so tests
// can simulate a FUSE filesystem.
var fuseDetect = isFUSEPath

// resolveHashStrategy decides whether to use mmap based on platform support and
// the MKBRR_HASH_MMAP / MKBRR_MMAP_WINDOW environment overrides. mmap is the
// default on unix; it can be disabled with MKBRR_HASH_MMAP=0 (escape hatch for
// unusual filesystems) and its window tuned with MKBRR_MMAP_WINDOW=<bytes>.
// NewPieceHasher additionally disables mmap on FUSE mounts (where it reads
// slower than read(); see isFUSEPath) unless MKBRR_HASH_MMAP explicitly forced
// it on.
func resolveHashStrategy() hashStrategy {
	s := hashStrategy{useMmap: mmapSupported, mmapWindow: defaultMmapWindow}

	switch os.Getenv("MKBRR_HASH_MMAP") {
	case "0", "false", "off", "no":
		s.useMmap = false
	case "1", "true", "on", "yes":
		s.useMmap = mmapSupported // never enable where unsupported
		s.forced = mmapSupported
	}

	if v := os.Getenv("MKBRR_MMAP_WINDOW"); v != "" {
		if w, err := strconv.ParseInt(v, 10, 64); err == nil && w > 0 {
			s.mmapWindow = w
		}
	}
	return s
}

// newChunkReader builds the chunk reader for one worker. When mmap is active no
// pooled buffer is taken (and none is allocated), which is the bulk of the
// allocation reduction over the read() path.
func (s hashStrategy) newChunkReader(pool *sync.Pool) chunkReader {
	if s.useMmap && mmapSupported {
		return newMmapChunkReader(s.mmapWindow)
	}
	buf, _ := pool.Get().(*[]byte)
	return &bufferedChunkReader{buf: buf, pool: pool}
}
