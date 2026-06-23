//go:build unix

package torrent

import (
	"io"

	"golang.org/x/sys/unix"
)

// mmapSupported reports whether the mmap hashing path is compiled in.
const mmapSupported = true

// mmapChunkReader feeds file bytes into a hash via mmap instead of read().
//
// It maps the file in page-aligned windows of at most `window` bytes, feeds
// each window's requested span into the hash, and unmaps it before advancing.
// This avoids one read() syscall and one kernel->user copy per chunk — the copy
// is where sys CPU concentrates on FUSE/network filesystems — while the
// per-window unmap keeps resident memory bounded to ~window per worker.
//
// A truncation of the file while it is mapped would fault on access (SIGBUS).
// Callers that use this reader run with debug.SetPanicOnFault enabled and
// recover the resulting panic, turning a mid-hash file change into a clean
// error rather than a crash; the per-window unmap is deferred so the mapping is
// released during that unwind.
type mmapChunkReader struct {
	window   int64
	pageMask int64
}

func newMmapChunkReader(window int64) chunkReader {
	ps := int64(unix.Getpagesize())
	if window < ps {
		window = ps
	}
	return &mmapChunkReader{window: window, pageMask: ps - 1}
}

func (m *mmapChunkReader) feed(h io.Writer, fr *fileReader, start, length int64) error {
	pos := start
	remaining := length
	fd := int(fr.file.Fd())
	for remaining > 0 {
		span := min(remaining, m.window)
		// mmap offset must be page-aligned; map from the aligned offset and
		// skip the leading slack when feeding the hash.
		aligned := pos &^ m.pageMask
		slack := pos - aligned
		mapLen := int(slack + span)

		if err := func() error {
			data, err := unix.Mmap(fd, aligned, mapLen, unix.PROT_READ, unix.MAP_SHARED)
			if err != nil {
				return err
			}
			defer func() { _ = unix.Munmap(data) }()
			_ = unix.Madvise(data, unix.MADV_SEQUENTIAL)
			_, werr := h.Write(data[slack : slack+span])
			return werr
		}(); err != nil {
			return err
		}

		pos += span
		remaining -= span
	}
	// keep fr.position coherent in case a later piece falls back to buffered reads
	fr.position = pos
	return nil
}

func (m *mmapChunkReader) release() {}
