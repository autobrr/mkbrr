package torrent

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"runtime"
	"sync"
)

type pieceHasher struct {
	pieces     [][]byte
	pieceLen   int64
	files      []fileEntry
	display    Displayer

	ch chan workHashUnit
	wg sync.WaitGroup
}

// optimizeForWorkload determines optimal read buffer size and number of worker goroutines
// based on the characteristics of input files (size and count). It considers:
// - single vs multiple files
// - average file size
// - system CPU count
func (h *pieceHasher) optimizeForWorkload() int {
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
	numWorkers = int(min(int64(numWorkers), int64(len(h.pieces))))
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

	return h.hashFiles()
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer) *pieceHasher {
	return &pieceHasher{
		pieces:   make([][]byte, numPieces),
		pieceLen: pieceLen,
		files:    files,
		display:  display,
	}
}

type workHashUnit struct {
	id int
	b  io.Reader
}

func (h *pieceHasher) runPieceWorkers() int {
	workers := h.optimizeForWorkload()

	// create channel before starting goroutines
	h.ch = make(chan workHashUnit, workers*64)
	for i := 0; i < workers; i++ {
		go func(ch <-chan workHashUnit) {
			r := bufio.NewReaderSize(io.Reader{}, int(max(h.pieceLen*int64(workers), int64(4<<20))))
			h := sha1.New()
			for w := range ch { // use local ch instead of h.ch
				h.Reset()
				r.Reset(w.b)
				io.Copy(h, r)
				h.pieces[w.id] = h.Sum(nil)
				h.wg.Done()
			}
		}(h.ch)
	}

	return workers
}

func (h *pieceHasher) hashFiles() error {
	b := h.bufferPool.Get().(*bytes.Buffer)
	workers := h.runPieceWorkers()
	defer close(h.ch)

	piece := 0
	lastRead := int64(0)

	reader = make([]io.Reader, 0)
	h.wg.Add(len(h.pieces))
	for i := 0; i < len(h.files); i++ {
		if err := func() error {
			f, err := os.OpenFile(h.files[i].path, os.O_RDONLY, 0)
			if err != nil {
				return err
			}

			defer f.Close()
			
			read := int64(0)
			fileSize := int64(h.files[i].length)
			for {
				toRead := min(h.pieceLen-lastRead, fileSize-read)
				if toRead == 0 {
					break
				}

				reader = append(reader, io.NewSectionReader(f, read, toRead))
				lastRead += toRead
				read += toRead

				if lastRead != h.pieceLen {
					continue
				}

				h.ch <- workHashUnit{id: piece, b: io.MultiReader(reader...)}
				piece++
				lastRead = 0
				reader = nil
			}

			if i == len(h.files)-1 && piece == len(h.pieces)-1 {
				h.ch <- workHashUnit{id: piece, b: io.MultiReader(reader...)}
				piece++
			}

			return nil
		}(); err != nil {
			return err
		}
	}

	if piece != len(h.pieces) {
		return fmt.Errorf("unable to create anticipated pieces %d/%d", piece, len(h.pieces))
	}

	h.wg.Wait()
	return nil
}
