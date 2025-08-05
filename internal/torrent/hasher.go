package torrent

import (
	"bufio"
	"crypto/sha1"
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
	startTime  time.Time
	lastUpdate time.Time
	display    Displayer
	bufferPool *sync.Pool
	pieces     [][]byte
	files      []fileEntry
	pieceLen   int64
	numPieces  int
	readSize   int
	totalSize  int64 // Pre-calculated total size

	// Cache for file positions per worker to minimize seeking
	filePositions map[string]int64

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

	// initialize buffer pool with much larger buffers
	h.bufferPool = &sync.Pool{
		New: func() interface{} {
			// Use 1MB buffers by default, or piece size if smaller
			bufSize := 1 << 20 // 1MB
			if h.pieceLen < int64(bufSize) {
				bufSize = int(h.pieceLen)
			}
			buf := make([]byte, bufSize)
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
	go func() {
		for {
			completed := atomic.LoadUint64(&completedPieces)
			if completed >= uint64(h.numPieces) {
				break
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

// hashPieceRange processes and hashes a specific range of pieces assigned to a worker.
// Optimized for sequential reading when possible to minimize seeking.
func (h *pieceHasher) hashPieceRange(startPiece, endPiece int, completedPieces *uint64) error {
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)

	openFiles := make(map[string]*os.File)
	filePositions := make(map[string]int64)
	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()

	totalSize := h.totalSize
	var localBytesProcessed int64

	// Check if we can do sequential optimization for large ranges
	if endPiece-startPiece > 10 && h.canOptimizeSequential(startPiece, endPiece) {
		return h.hashPieceRangeSequential(startPiece, endPiece, completedPieces, buf, &localBytesProcessed)
	}

	// Standard piece-by-piece processing with position tracking
	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceEnd := pieceOffset + h.pieceLen
		if pieceEnd > totalSize {
			pieceEnd = totalSize
		}

		// Create fresh hasher for each piece to avoid pool corruption
		hasher := sha1.New()

		if pieceOffset >= totalSize {
			h.pieces[pieceIndex] = hasher.Sum(nil)
			continue
		}

		if err := h.hashSinglePieceOptimized(pieceOffset, pieceEnd, hasher, buf, openFiles, filePositions, &localBytesProcessed); err != nil {
			return err
		}

		h.pieces[pieceIndex] = hasher.Sum(nil)
	}

	atomic.AddInt64(&h.bytesProcessed, localBytesProcessed)
	atomic.AddUint64(completedPieces, uint64(endPiece-startPiece))
	return nil
}

// hashSinglePieceOptimized - track file positions to minimize seeking with buffered I/O
func (h *pieceHasher) hashSinglePieceOptimized(pieceStart, pieceEnd int64, hasher hash.Hash, buf []byte, openFiles map[string]*os.File, filePositions map[string]int64, localBytes *int64) error {
	// Find intersecting files and process them in offset order (they should already be sorted)
	for _, file := range h.files {
		// Quick reject: file completely before or after piece
		if file.offset+file.length <= pieceStart || file.offset >= pieceEnd {
			continue
		}

		// Calculate intersection bounds
		readStart := maxInt64(pieceStart, file.offset) - file.offset
		readEnd := minInt64(pieceEnd, file.offset+file.length) - file.offset

		if readEnd <= readStart {
			continue
		}

		// Get file handle
		f := openFiles[file.path]
		if f == nil {
			var err error
			f, err = os.Open(file.path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", file.path, err)
			}
			openFiles[file.path] = f
			filePositions[file.path] = -1 // Initialize position tracker
		}

		// Only seek if we're not already at the right position
		currentPos := filePositions[file.path]
		if currentPos != readStart {
			if _, err := f.Seek(readStart, 0); err != nil {
				return fmt.Errorf("seek failed in %s: %w", file.path, err)
			}
			filePositions[file.path] = readStart
		}

		bytesToRead := readEnd - readStart

		// For small reads, use direct reading. For larger reads, consider buffered I/O
		if bytesToRead > int64(len(buf))*2 {
			// Use buffered reader for larger sequential reads
			reader := bufio.NewReaderSize(f, len(buf))
			for bytesToRead > 0 {
				chunkSize := int64(len(buf))
				if bytesToRead < chunkSize {
					chunkSize = bytesToRead
				}

				n, err := io.ReadFull(reader, buf[:chunkSize])
				if err == io.ErrUnexpectedEOF {
					if n > 0 {
						hasher.Write(buf[:n])
						*localBytes += int64(n)
						filePositions[file.path] += int64(n)
					}
					break
				} else if err != nil {
					return fmt.Errorf("buffered read failed from %s: %w", file.path, err)
				}

				hasher.Write(buf[:n])
				bytesToRead -= int64(n)
				*localBytes += int64(n)
				filePositions[file.path] += int64(n)
			}
		} else {
			// Direct reading for smaller chunks
			for bytesToRead > 0 {
				chunkSize := int64(len(buf))
				if bytesToRead < chunkSize {
					chunkSize = bytesToRead
				}

				n, err := io.ReadFull(f, buf[:chunkSize])
				if err == io.ErrUnexpectedEOF {
					if n > 0 {
						hasher.Write(buf[:n])
						*localBytes += int64(n)
						filePositions[file.path] += int64(n)
					}
					break
				} else if err != nil {
					return fmt.Errorf("read failed from %s: %w", file.path, err)
				}

				hasher.Write(buf[:n])
				bytesToRead -= int64(n)
				*localBytes += int64(n)
				filePositions[file.path] += int64(n)
			}
		}
	}
	return nil
}

// canOptimizeSequential determines if we can read files sequentially for better performance
func (h *pieceHasher) canOptimizeSequential(startPiece, endPiece int) bool {
	// Check if most pieces come from a single large file
	pieceStart := int64(startPiece) * h.pieceLen
	pieceEnd := int64(endPiece) * h.pieceLen
	if pieceEnd > h.totalSize {
		pieceEnd = h.totalSize
	}

	// Find files that cover this range
	coveringFiles := 0
	totalCoverage := int64(0)

	for _, file := range h.files {
		if file.offset+file.length <= pieceStart || file.offset >= pieceEnd {
			continue
		}
		coveringFiles++

		// Calculate intersection
		start := maxInt64(pieceStart, file.offset)
		end := minInt64(pieceEnd, file.offset+file.length)
		totalCoverage += end - start
	}

	// If one or two files cover most of the range, sequential reading is beneficial
	return coveringFiles <= 2 && totalCoverage >= (pieceEnd-pieceStart)*3/4
}

// hashPieceRangeSequential optimizes for sequential reading of large file ranges
func (h *pieceHasher) hashPieceRangeSequential(startPiece, endPiece int, completedPieces *uint64, buf []byte, localBytes *int64) error {
	// Process pieces sequentially with minimal seeking
	openFiles := make(map[string]*os.File)
	defer func() {
		for _, f := range openFiles {
			f.Close()
		}
	}()

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceEnd := pieceOffset + h.pieceLen
		if pieceEnd > h.totalSize {
			pieceEnd = h.totalSize
		}

		// Create fresh hasher for each piece
		hasher := sha1.New()

		if pieceOffset >= h.totalSize {
			h.pieces[pieceIndex] = hasher.Sum(nil)
			continue
		}

		// Read piece data sequentially
		for _, file := range h.files {
			if file.offset+file.length <= pieceOffset || file.offset >= pieceEnd {
				continue
			}

			// Calculate what we need from this file
			readStart := maxInt64(pieceOffset, file.offset) - file.offset
			readEnd := minInt64(pieceEnd, file.offset+file.length) - file.offset

			if readEnd <= readStart {
				continue
			}

			// Get file handle
			f := openFiles[file.path]
			if f == nil {
				var err error
				f, err = os.Open(file.path)
				if err != nil {
					return fmt.Errorf("failed to open %s: %w", file.path, err)
				}
				openFiles[file.path] = f
			}

			// Seek and read
			if _, err := f.Seek(readStart, 0); err != nil {
				return fmt.Errorf("seek failed: %w", err)
			}

			remaining := readEnd - readStart
			for remaining > 0 {
				toRead := remaining
				if toRead > int64(len(buf)) {
					toRead = int64(len(buf))
				}

				n, err := f.Read(buf[:toRead])
				if err != nil && err != io.EOF {
					return fmt.Errorf("read failed: %w", err)
				}
				if n == 0 {
					break
				}

				hasher.Write(buf[:n])
				remaining -= int64(n)
				*localBytes += int64(n)
			}
		}

		h.pieces[pieceIndex] = hasher.Sum(nil)
	}

	atomic.AddInt64(&h.bytesProcessed, *localBytes)
	atomic.AddUint64(completedPieces, uint64(endPiece-startPiece))
	return nil
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer, failOnSeasonPackWarning bool) *pieceHasher {
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, pieceLen)
			return buf
		},
	}

	// Pre-calculate total size once
	var totalSize int64
	for _, f := range files {
		totalSize += f.length
	}

	return &pieceHasher{
		pieces:                  make([][]byte, numPieces),
		pieceLen:                pieceLen,
		numPieces:               numPieces,
		files:                   files,
		totalSize:               totalSize,
		display:                 display,
		bufferPool:              bufferPool,
		failOnSeasonPackWarning: failOnSeasonPackWarning,
	}
}

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maxInt64 returns the larger of two int64 values
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// minInt64 returns the smaller of two int64 values
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
