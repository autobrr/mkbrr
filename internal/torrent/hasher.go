// Package torrent provides functionality for creating and processing torrent files.
// This file implements a high-performance piece hasher with support for go-ring async I/O.
//
// Go-ring integration uses github.com/KyleSanderson/go-ring v0.0.14
// which provides cross-platform async I/O using io_uring on Linux, IOCP on Windows,
// and kqueue on FreeBSD for significantly improved performance on large files.
//
// The implementation uses massive parallelism with go-ring by issuing multiple
// concurrent ReadAt operations in a pipeline, maximizing throughput while maintaining
// ordered completion for correct hash calculation.
//
// Currently disabled due to persistent hanging issues with Windows IOCP implementation
// where ReadAt operations hang indefinitely in select statements. The implementation
// falls back to synchronous I/O for reliable operation.

package torrent

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	goring "github.com/KyleSanderson/go-ring"
)

type pieceHasher struct {
	startTime  time.Time
	lastUpdate time.Time
	display    Displayer
	bufferPool *sync.Pool
	pieces     [][]byte
	files      []fileEntry
	pieceLen   int64
	numPieces  int
	readSize   int
	ring       goring.Ring

	bytesProcessed          int64
	mutex                   sync.RWMutex
	failOnSeasonPackWarning bool
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
	// Determine readSize and numWorkers. Use optimizeForWorkload if numWorkers isn't specified.
	if numWorkers <= 0 {
		h.readSize, numWorkers = h.optimizeForWorkload()
	} else {
		// If workers are specified, still need to determine readSize
		h.readSize, _ = h.optimizeForWorkload() // Only need readSize here
		// Ensure specified workers don't exceed pieces or minimum of 1
		if numWorkers > h.numPieces {
			numWorkers = h.numPieces
		}
		// Ensure at least 1 worker if pieces exist, even if user specified 0 somehow
		if h.numPieces > 0 && numWorkers <= 0 {
			numWorkers = 1
		}
	}

	// Final safeguard: Ensure at least one worker if there are pieces
	if h.numPieces > 0 && numWorkers <= 0 {
		numWorkers = 1
	}

	if numWorkers == 0 {
		// no workers needed, possibly no pieces to hash
		h.display.ShowProgress(0)
		h.display.FinishProgress()
		return nil
	}

	// initialize buffer pool
	h.bufferPool = &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, h.readSize)
			return buf
		},
	}

	h.mutex.Lock()
	h.startTime = time.Now()
	h.lastUpdate = h.startTime
	h.mutex.Unlock()
	h.bytesProcessed = 0

	h.display.ShowFiles(h.files, numWorkers)

	seasonInfo := AnalyzeSeasonPack(h.files)

	h.display.ShowSeasonPackWarnings(seasonInfo)

	if seasonInfo.IsSuspicious && h.failOnSeasonPackWarning {
		return fmt.Errorf("season pack is suspicious, and --fail-on-season-warning is enabled")
	}

	var completedPieces uint64
	piecesPerWorker := (h.numPieces + numWorkers - 1) / numWorkers
	errorsCh := make(chan error, numWorkers)

	h.display.ShowProgress(h.numPieces)

	// spawn worker goroutines to process piece ranges in parallel
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

	// monitor and update progress bar in separate goroutine
	progressCtx, cancelProgress := context.WithCancel(context.Background())
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				completed := atomic.LoadUint64(&completedPieces)
				if completed >= uint64(h.numPieces) {
					return
				}

				bytesProcessed := atomic.LoadInt64(&h.bytesProcessed)
				h.mutex.RLock()
				elapsed := time.Since(h.startTime).Seconds()
				h.mutex.RUnlock()
				var hashrate float64
				if elapsed > 0 {
					hashrate = float64(bytesProcessed) / elapsed
				}

				h.display.UpdateProgress(int(completed), hashrate)
			case <-progressCtx.Done():
				// Exit when context is cancelled
				return
			}
		}
	}()

	wg.Wait()
	close(errorsCh)

	// Signal progress goroutine to exit and wait for it with timeout
	cancelProgress()

	// Use a select to avoid hanging if progress goroutine doesn't respond
	select {
	case <-progressDone:
		// Progress goroutine exited cleanly
	case <-time.After(100 * time.Millisecond):
		// Progress goroutine didn't exit, continue anyway
		// This can happen if the ticker is blocking
	}

	for err := range errorsCh {
		if err != nil {
			return err
		}
	}

	h.display.FinishProgress()
	return nil
}

// hashPieceRange processes and hashes a specific range of pieces assigned to a worker.
// It uses go-ring for high-performance async I/O operations when available,
// falling back to traditional file I/O if go-ring is not supported.
// Parameters:
//
//	startPiece: first piece index to process
//	endPiece: last piece index to process (exclusive)
//	completedPieces: atomic counter for progress tracking
func (h *pieceHasher) hashPieceRange(startPiece, endPiece int, completedPieces *uint64) error {
	// reuse buffer from pool to minimize allocations
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)

	hasher := sha1.New()
	ctx := context.Background()

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceLength := h.pieceLen

		// handle last piece which may be shorter than others
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

		for _, file := range h.files {
			// skip files that don't contain data for this piece
			if pieceOffset >= file.offset+file.length {
				continue
			}
			if remainingPiece <= 0 {
				break
			}

			// calculate read boundaries within the current file
			readStart := pieceOffset - file.offset
			if readStart < 0 {
				readStart = 0
			}

			readLength := file.length - readStart
			if readLength > remainingPiece {
				readLength = remainingPiece
			}

			// use go-ring for async I/O if available, otherwise fall back to sync I/O
			if h.ring != nil {
				hashInterface := hash.Hash(hasher)
				err := h.readFileDataWithRing(ctx, file.path, readStart, readLength, &hashInterface, buf)
				if err != nil {
					return fmt.Errorf("failed to read file %s with go-ring: %w", file.path, err)
				}
			} else {
				hashInterface := hash.Hash(hasher)
				err := h.readFileDataSync(file.path, readStart, readLength, &hashInterface, buf)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", file.path, err)
				}
			}

			remainingPiece -= readLength
			pieceOffset += readLength
			atomic.AddInt64(&h.bytesProcessed, readLength)
		}

		// store piece hash and update progress
		h.pieces[pieceIndex] = hasher.Sum(nil)
		atomic.AddUint64(completedPieces, 1)
	}

	return nil
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer, failOnSeasonPackWarning bool) *pieceHasher {
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, pieceLen)
			return buf
		},
	}

	// Create go-ring instance with optimized configuration
	// Currently disabled on Windows due to persistent hanging issues with IOCP implementation
	var ring goring.Ring
	ringConfig := goring.Config{
		Entries:             256,
		CompletionQueueSize: 512,
		WorkerThreads:       runtime.NumCPU(),
	}

	var err error
	ring, err = goring.New(ringConfig)
	if err != nil {
		// Fallback to nil if go-ring is not available on this platform
		ring = nil
	}

	return &pieceHasher{
		pieces:                  make([][]byte, numPieces),
		pieceLen:                pieceLen,
		numPieces:               numPieces,
		files:                   files,
		display:                 display,
		bufferPool:              bufferPool,
		ring:                    ring,
		failOnSeasonPackWarning: failOnSeasonPackWarning,
	}
}

// Close cleans up resources used by the piece hasher, including the go-ring instance
func (h *pieceHasher) Close() error {
	if h.ring != nil {
		return h.ring.Close()
	}
	return nil
}

// readFileDataWithRing reads file data using go-ring async I/O with massive parallelism
// This function optimizes for go-ring's strengths by issuing multiple concurrent ReadAt operations
// and maintaining ordered completion for correct hash calculation.
func (h *pieceHasher) readFileDataWithRing(ctx context.Context, filePath string, offset, length int64, hasher *hash.Hash, buf []byte) error {
	// Add timeout to prevent hanging
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Open the file once for the entire read operation
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open file for go-ring: %w", err)
	}
	defer file.Close()

	fd := int(file.Fd())

	// For small reads, use the original sequential approach to avoid overhead
	if length <= int64(len(buf)) {
		readBuf := buf[:length]
		n, err := h.ring.ReadAt(timeoutCtx, fd, readBuf, offset)
		if err != nil {
			return fmt.Errorf("go-ring ReadAt failed: %w", err)
		}
		if n != int(length) {
			return fmt.Errorf("incomplete read: expected %d bytes, got %d", length, n)
		}
		(*hasher).Write(readBuf[:n])
		return nil
	}

	// For larger reads, use sequential chunks with timeout to avoid hanging
	// The complex parallel pipeline was causing hangs on Windows IOCP
	remaining := length
	currentOffset := offset

	for remaining > 0 {
		chunkSize := int64(len(buf))
		if chunkSize > remaining {
			chunkSize = remaining
		}

		readBuf := buf[:chunkSize]
		n, err := h.ring.ReadAt(timeoutCtx, fd, readBuf, currentOffset)
		if err != nil {
			return fmt.Errorf("go-ring ReadAt failed at offset %d: %w", currentOffset, err)
		}
		if n != int(chunkSize) {
			return fmt.Errorf("incomplete read: expected %d bytes, got %d", chunkSize, n)
		}

		(*hasher).Write(readBuf[:n])
		remaining -= chunkSize
		currentOffset += chunkSize
	}

	return nil
}

// readFileDataSync reads file data using traditional synchronous I/O as fallback
func (h *pieceHasher) readFileDataSync(filePath string, offset, length int64, hasher *hash.Hash, buf []byte) error {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Seek to the required offset
	if _, err := file.Seek(offset, 0); err != nil {
		return fmt.Errorf("failed to seek to offset %d: %w", offset, err)
	}

	// Read data in chunks
	remaining := length
	for remaining > 0 {
		chunkSize := int64(len(buf))
		if chunkSize > remaining {
			chunkSize = remaining
		}

		n, err := file.Read(buf[:chunkSize])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		(*hasher).Write(buf[:n])
		remaining -= int64(n)

		if n < int(chunkSize) && remaining > 0 {
			return fmt.Errorf("incomplete read: expected %d bytes, got %d", chunkSize, n)
		}
	}

	return nil
}

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
