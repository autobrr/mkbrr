//go:build !unix

package torrent

import (
	"errors"
	"io"
)

// mmapSupported reports whether the mmap hashing path is compiled in. On
// non-unix platforms (e.g. Windows) it is not, and hashing uses the buffered
// read() path exclusively.
const mmapSupported = false

var errMmapUnsupported = errors.New("mmap hashing not supported on this platform")

type mmapChunkReader struct{}

func newMmapChunkReader(window int64) chunkReader { return &mmapChunkReader{} }

func (m *mmapChunkReader) feed(h io.Writer, fr *fileReader, start, length int64) error {
	return errMmapUnsupported
}

func (m *mmapChunkReader) release() {}
