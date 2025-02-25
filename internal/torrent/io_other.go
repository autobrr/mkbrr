//go:build !linux
// +build !linux

package torrent

import (
	"os"
)

// ioReader is an interface for reading files
type ioReader interface {
	Read(offset int64, buf []byte) (int, error)
	Close() error
}

// NewIOReader creates a standard file reader on non-Linux platforms
func NewIOReader(path string) (ioReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &stdReader{file: f}, nil
}

// stdReader implements ioReader using standard os.File
type stdReader struct {
	file *os.File
}

// Read implements the ioReader interface using standard file IO
func (r *stdReader) Read(offset int64, buf []byte) (int, error) {
	// Seek to the offset
	_, err := r.file.Seek(offset, 0)
	if err != nil {
		return 0, err
	}
	// Read the data
	return r.file.Read(buf)
}

// Close closes the file
func (r *stdReader) Close() error {
	return r.file.Close()
} 