//go:build unix

package torrent

import (
	"io"

	"golang.org/x/sys/unix"
)

// mmapSupported reports whether the mmap hashing path is compiled in.
const mmapSupported = true

// maxFallbackBuf caps the lazily-allocated buffer used only when mmap fails and
// feed falls back to read(); keeps a large tuned window from forcing a large
// allocation on the rare fallback path.
const maxFallbackBuf = 8 << 20

// mmapFunc is the mmap syscall, indirected so tests can force the fallback path.
var mmapFunc = unix.Mmap

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
// released during that unwind. A file that shrank *before* hashing (so the
// requested span runs past EOF without ever touching a fully-unbacked page) is
// caught up front by the fstat size check, matching the read() path's
// io.ErrUnexpectedEOF instead of silently hashing the kernel's zero-fill.
type mmapChunkReader struct {
	window   int64
	pageMask int64

	// cached on-disk size for the fd currently being fed (single-entry cache;
	// a worker hashes consecutive pieces of the same file, so this is one
	// fstat per file rather than per piece).
	sizeFd   int
	size     int64
	haveSize bool

	// buffer for the read() fallback; allocated lazily only if mmap fails, so
	// the common path stays allocation-free.
	fallbackBuf []byte
}

func newMmapChunkReader(window int64) chunkReader {
	ps := int64(unix.Getpagesize())
	if window < ps {
		window = ps
	}
	// Round down to a whole number of pages so successive windows stay
	// page-aligned (otherwise pos drifts off alignment after the first window
	// and every boundary page gets re-mapped), and cap it so slack+window
	// cannot overflow a 32-bit int in the mmap length below.
	window -= window % ps
	const maxWindow = 1 << 30 // 1 GiB; far more than useful, safe on 32-bit int
	if window > maxWindow {
		window = maxWindow - (maxWindow % ps)
	}
	return &mmapChunkReader{window: window, pageMask: ps - 1}
}

func (m *mmapChunkReader) fileSize(fd int) (int64, error) {
	if m.haveSize && m.sizeFd == fd {
		return m.size, nil
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return 0, err
	}
	m.sizeFd, m.size, m.haveSize = fd, st.Size, true
	return m.size, nil
}

func (m *mmapChunkReader) feed(h io.Writer, fr *fileReader, start, length int64) error {
	fd := int(fr.file.Fd())

	// fileEntry.length is captured at scan time; if the file shrank before
	// hashing, the requested span can run past the real EOF. When the shortfall
	// stays inside the last backed page the kernel zero-fills it and no SIGBUS
	// fires, so without this check mmap would silently hash zeros and bake a
	// wrong piece hash into the torrent. The read() path returns
	// io.ErrUnexpectedEOF here; match it.
	size, err := m.fileSize(fd)
	if err != nil {
		return err
	}
	if start+length > size {
		return io.ErrUnexpectedEOF
	}

	pos := start
	remaining := length
	for remaining > 0 {
		span := min(remaining, m.window)
		// mmap offset must be page-aligned; map from the aligned offset and
		// skip the leading slack when feeding the hash.
		aligned := pos &^ m.pageMask
		slack := pos - aligned
		mapLen := int(slack + span)

		data, merr := mmapFunc(fd, aligned, mapLen, unix.PROT_READ, unix.MAP_SHARED)
		if merr != nil {
			// mmap can fail on some filesystems (certain FUSE/network/overlay
			// mounts) where read() still works. Fall back to a buffered read of
			// this span so create still succeeds there instead of aborting.
			if rerr := m.readSpan(h, fr, pos, span); rerr != nil {
				return rerr
			}
			pos += span
			remaining -= span
			continue
		}

		werr := func() error {
			defer func() { _ = unix.Munmap(data) }()
			_ = unix.Madvise(data, unix.MADV_SEQUENTIAL)
			_, e := h.Write(data[slack : slack+span])
			return e
		}()
		if werr != nil {
			return werr
		}

		pos += span
		remaining -= span
	}
	return nil
}

// readSpan feeds [start, start+length) into h using read() instead of mmap.
// Used only as the fallback when mmapFunc fails. The size check in feed has
// already confirmed the bytes exist, so a short read here means the file was
// truncated mid-hash and is reported as io.ErrUnexpectedEOF.
func (m *mmapChunkReader) readSpan(h io.Writer, fr *fileReader, start, length int64) error {
	if m.fallbackBuf == nil {
		m.fallbackBuf = make([]byte, min(m.window, maxFallbackBuf))
	}
	buf := m.fallbackBuf
	off := start
	remaining := length
	for remaining > 0 {
		n := min(int64(len(buf)), remaining)
		read, err := fr.file.ReadAt(buf[:n], off)
		if read > 0 {
			if _, werr := h.Write(buf[:read]); werr != nil {
				return werr
			}
			off += int64(read)
			remaining -= int64(read)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	if remaining > 0 {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (m *mmapChunkReader) release() {}
