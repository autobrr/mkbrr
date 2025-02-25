package torrent

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Add a new constant for batch size
const (
	maxBatchSize = 16 // Maximum number of concurrent I/O operations per piece
)

type pieceHasher struct {
	pieces     [][]byte
	pieceLen   int64
	numPieces  int
	files      []fileEntry
	display    Displayer
	bufferPool *sync.Pool
	readSize   int
}

// optimizeForWorkload determines optimal read buffer size and number of worker goroutines
// based on the characteristics of input files (size and count). It considers:
// - single vs multiple files
// - average file size
// - system CPU count
// returns readSize (buffer size for reading) and numWorkers (concurrent goroutines)
func (h *pieceHasher) optimizeForWorkload() (int, int) {
	if len(h.files) == 0 {
		return 0, 0
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

	var readSize, numWorkers int

	// optimize buffer size and worker count based on file characteristics
	switch {
	case len(h.files) == 1:
		if totalSize < 1<<20 {
			readSize = 64 << 10 // 64 KiB for very small files
			numWorkers = 1
		} else if totalSize < 1<<30 { // < 1 GiB
			readSize = 4 << 20 // 4 MiB
			numWorkers = runtime.NumCPU()
		} else {
			readSize = 8 << 20                // 8 MiB for large files
			numWorkers = runtime.NumCPU() * 2 // over-subscription for better I/O utilization
		}
	case avgFileSize < 1<<20: // avg < 1 MiB
		readSize = 256 << 10 // 256 KiB
		numWorkers = runtime.NumCPU()
	case avgFileSize < 10<<20: // avg < 10 MiB
		readSize = 1 << 20 // 1 MiB
		numWorkers = runtime.NumCPU()
	case avgFileSize < 1<<30: // avg < 1 GiB
		readSize = 4 << 20 // 4 MiB
		numWorkers = runtime.NumCPU() * 2
	default: // avg >= 1 GiB
		readSize = 8 << 20 // 8 MiB
		numWorkers = runtime.NumCPU() * 2
	}

	// ensure we don't create more workers than pieces to process
	if numWorkers > h.numPieces {
		numWorkers = h.numPieces
	}
	return readSize, numWorkers
}

// hashPieces coordinates the parallel hashing of all pieces in the torrent.
// It initializes a buffer pool, creates worker goroutines, and manages progress tracking.
// The pieces are distributed evenly across the specified number of workers.
// Returns an error if any worker encounters issues during hashing.
func (h *pieceHasher) hashPieces(numWorkers int) error {
	if numWorkers <= 0 && len(h.files) > 0 {
		return errors.New("number of workers must be greater than zero when files are present")
	}

	h.readSize, numWorkers = h.optimizeForWorkload()

	if numWorkers == 0 {
		h.display.ShowProgress(0)
		h.display.FinishProgress()
		return nil
	}

	h.bufferPool = &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, h.readSize)
			return buf
		},
	}

	var completedPieces uint64
	piecesPerWorker := (h.numPieces + numWorkers - 1) / numWorkers
	errorsCh := make(chan error, numWorkers)

	h.display.ShowProgress(h.numPieces)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		start := i * piecesPerWorker
		end := start + piecesPerWorker
		if end > h.numPieces {
			end = h.numPieces
		}

		wg.Add(1)
		go func(startPiece, endPiece int) {
			defer wg.Done()
			if err := h.hashPieceRange(startPiece, endPiece, &completedPieces); err != nil {
				errorsCh <- err
			}
		}(start, end)
	}

	go func() {
		for {
			completed := atomic.LoadUint64(&completedPieces)
			if completed >= uint64(h.numPieces) {
				break
			}
			h.display.UpdateProgress(int(completed))
			time.Sleep(200 * time.Millisecond)
		}
	}()

	wg.Wait()
	close(errorsCh)

	for err := range errorsCh {
		if err != nil {
			return err
		}
	}

	h.display.FinishProgress()
	return nil
}

// hashPieceRange optimized for batch I/O operations
func (h *pieceHasher) hashPieceRange(startPiece, endPiece int, completedPieces *uint64) error {
	// Get multiple buffers for batch operations
	buffers := make([][]byte, maxBatchSize)
	for i := range buffers {
		buffers[i] = h.bufferPool.Get().([]byte)
	}
	defer func() {
		for i := range buffers {
			h.bufferPool.Put(buffers[i])
		}
	}()

	hasher := sha1.New()
	readers := make(map[string]*fileReaderState)
	defer func() {
		for _, r := range readers {
			r.reader.Close()
		}
	}()

	// Process each piece
	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceLength := h.pieceLen

		// Handle last piece which may be shorter
		if pieceIndex == h.numPieces-1 {
			var totalLength int64
			for _, f := range h.files {
				totalLength += f.length
			}
			remaining := totalLength - pieceOffset
			if remaining < pieceLength {
				pieceLength = remaining
			}
		}

		hasher.Reset()
		remainingPiece := pieceLength

		// Prepare read operations for this piece
		type readOp struct {
			file       fileEntry
			readStart  int64
			readLength int64
			bufferIdx  int
		}

		// Plan all read operations for this piece
		var readOps []readOp
		currentOffset := pieceOffset
		
		for _, file := range h.files {
			// Skip files that don't contain data for this piece
			if currentOffset >= file.offset+file.length {
				continue
			}
			if remainingPiece <= 0 {
				break
			}

			// Calculate read boundaries within the current file
			readStart := currentOffset - file.offset
			if readStart < 0 {
				readStart = 0
			}

			readLength := file.length - readStart
			if readLength > remainingPiece {
				readLength = remainingPiece
			}

			// Split large reads into buffer-sized chunks
			remaining := readLength
			fileReadStart := readStart
			
			for remaining > 0 {
				chunkSize := int64(h.readSize)
				if remaining < chunkSize {
					chunkSize = remaining
				}
				
				readOps = append(readOps, readOp{
					file:       file,
					readStart:  fileReadStart,
					readLength: chunkSize,
					bufferIdx:  len(readOps) % maxBatchSize,
				})
				
				fileReadStart += chunkSize
				remaining -= chunkSize
				remainingPiece -= chunkSize
				currentOffset += chunkSize
			}
		}

		// Process read operations in batches
		for i := 0; i < len(readOps); i += maxBatchSize {
			batchEnd := i + maxBatchSize
			if batchEnd > len(readOps) {
				batchEnd = len(readOps)
			}
			
			// Submit batch of read operations
			results := make(chan struct {
				data []byte
				err  error
			}, batchEnd-i)
			
			for j := i; j < batchEnd; j++ {
				op := readOps[j]
				buffer := buffers[op.bufferIdx][:op.readLength]
				
				// Get or create reader for this file
				reader, ok := readers[op.file.path]
				if !ok {
					ioReader, err := NewIOReader(op.file.path)
					if err != nil {
						return fmt.Errorf("failed to open file %s: %w", op.file.path, err)
					}
					reader = &fileReaderState{
						reader:   ioReader,
						position: 0,
						length:   op.file.length,
					}
					readers[op.file.path] = reader
				}
				
				// Submit read operation asynchronously
				go func(r ioReader, offset int64, buf []byte, idx int) {
					n, err := r.Read(offset, buf)
					if err != nil {
						results <- struct {
							data []byte
							err  error
						}{nil, fmt.Errorf("failed to read file: %w", err)}
						return
					}
					
					results <- struct {
						data []byte
						err  error
					}{buf[:n], nil}
				}(reader.reader, op.readStart, buffer, j-i)
			}
			
			// Process results as they complete
			for j := i; j < batchEnd; j++ {
				result := <-results
				if result.err != nil {
					return result.err
				}
				
				// Update hash with read data
				hasher.Write(result.data)
			}
		}

		// Store piece hash and update progress
		h.pieces[pieceIndex] = hasher.Sum(nil)
		atomic.AddUint64(completedPieces, 1)
	}

	return nil
}

// fileReaderState tracks the state of an open file reader
type fileReaderState struct {
	reader   ioReader
	position int64
	length   int64
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer) *pieceHasher {
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, pieceLen)
			return buf
		},
	}
	return &pieceHasher{
		pieces:     make([][]byte, numPieces),
		pieceLen:   pieceLen,
		numPieces:  numPieces,
		files:      files,
		display:    display,
		bufferPool: bufferPool,
	}
}

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
