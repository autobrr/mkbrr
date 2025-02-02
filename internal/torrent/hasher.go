package torrent

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type pieceHasher struct {
	pieces     [][]byte
	pieceLen   int64
	files      []fileEntry
	display    Displayer
	bufferPool *sync.Pool
	ch         chan *[]byte
}

// optimizeForWorkload determines optimal read buffer size and number of worker goroutines
// based on the characteristics of input files (size and count). It considers:
// - single vs multiple files
// - average file size
// - system CPU count
func (h *pieceHasher) optimizeForWorkload() (int) {
	if len(h.files) == 0 {
		return 0
	}

	// calculate total and maximum file sizes for optimization decisions
	var totalSize int64
	maxFileSize := int64(0)
	for _, f := range h.files {
		totalSize += f.length
		if f.length > maxFileSize {
			maxFileSize = f.length
		}
	}
	avgFileSize := totalSize / int64(len(h.files))

	var numWorkers int

	// adjust buffer size and worker count based on file characteristics:
	// - smaller buffers and fewer workers for small files
	// - larger buffers and more workers for large files
	switch {
	case len(h.files) == 1:
		if totalSize < 1<<20 {
			numWorkers = 1
		} else if totalSize < 1<<30 {
			numWorkers = 2
		} else {
			numWorkers = 4
		}
	case avgFileSize < 1<<20:
		numWorkers = int(min(int64(8), int64(runtime.NumCPU())))
	case avgFileSize < 10<<20:
		numWorkers = int(min(int64(4), int64(runtime.NumCPU())))
	default:
		numWorkers = int(min(int64(2), int64(runtime.NumCPU())))
	}

	// ensure we don't create more workers than pieces to process
	if numWorkers > len(h.pieces) {
		numWorkers = len(h.pieces)
	}
	return numWorkers
}

// hashPieces coordinates the parallel hashing of all pieces in the torrent.
// It initializes a buffer pool, creates worker goroutines, and manages progress tracking.
// The pieces are distributed evenly across the specified number of workers.
// Returns an error if any worker encounters issues during hashing.
func (h *pieceHasher) hashPieces(numWorkers int) error {
	if numWorkers <= 0 && len(h.files) > 0 {
		return errors.New("number of workers must be greater than zero when files are present")
	}

	numWorkers = h.optimizeForWorkload()

	if numWorkers == 0 {
		// no workers needed, possibly no pieces to hash
		h.display.ShowProgress(0)
		h.display.FinishProgress()
		return nil
	}

	// initialize buffer pool
	h.bufferPool = &sync.Pool{
		New: func() interface{} {
			return sha1.New()
		},
	}

	return h.hashFiles()
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer) *pieceHasher {
	return &pieceHasher{
		pieces:    make([][]byte, numPieces),
		pieceLen:  pieceLen,
		files:     files,
		display:   display,
		ch:        make(chan *[]byte, 64), // should be numWorkers*3 at minimum to get good velocity.
	}
}

func (h *pieceHasher) hashPiece(piece int, hasher *hash.Hash) {
	defer h.bufferPool.Put(hasher)
	defer (*hasher).Reset()
	h.pieces[piece] = (*hasher).Sum(nil)
}

func (h *pieceHasher) hashFiles() error {
	hasher := h.bufferPool.Get().(*hash.Hash)

	workers := h.optimizeForWorkload()
	piece := 0
	lastRead := 0
	var wg sync.WaitGroup
	for i := 0; i < len(f.files); i++ {
		if err := func () error {
			of, err := os.OpenFile(f.files[i].Path, os.O_RDONLY, 0)
			if err != nil {
				return err
			}
	
			defer of.Close()
			r := bufio.NewReaderSize(of, h.piecesLen * workers)
			read := 0
			for {
				toRead := min(h.PieceLen - lastRead, f.files[i].length - read)
				if err := io.CopyN(*hasher, r, toRead); err != nil {
					if err == io.EOF {
						break
					}

					return err
				}

				lastRead += toRead
				if lastRead == h.PieceLen || i == len(f.files)-1 && piece == len(h.pieces)-1 {
					wg.Add(1)
					go func(p int, h *hash.Hash) {
						h.hashPiece(p, h)
						wg.Done()
					}(piece, hasher)
					piece++
					lastRead = 0
					hasher = h.bufferPool.Get().(*hash.Hash)
				}
			}
		}(); err != nil {
			return err
		}
	}
	wg.Wait()
	return nil
}
	
