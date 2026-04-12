package torrent

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type pieceHasher struct {
	display          Displayer
	bufferPool       *sync.Pool
	pieces           [][]byte
	pieceHashStorage []byte
	files            []fileEntry
	pieceLen         int64
	numPieces        int
	readSize         int
	totalSize        int64
	lastPieceLength  int64
	pieceStartFiles  []int

	startTime               time.Time
	bytesProcessed          int64
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

	maxFileSize := int64(0)
	for _, f := range h.files {
		if f.length > maxFileSize {
			maxFileSize = f.length
		}
	}
	avgFileSize := h.totalSize / int64(len(h.files))

	var readSize, numWorkers int

	// optimize buffer size and worker count based on file characteristics
	switch {
	case len(h.files) == 1:
		if h.totalSize < 1<<20 {
			readSize = 64 << 10 // 64 KiB for very small files
			numWorkers = 1
		} else if h.totalSize < 1<<30 { // < 1 GiB
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

	h.startTime = time.Now()
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
	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				completed := atomic.LoadUint64(&completedPieces)
				bytesProcessed := atomic.LoadInt64(&h.bytesProcessed)
				elapsed := time.Since(h.startTime).Seconds()

				var hashrate float64
				if elapsed > 0 {
					hashrate = float64(bytesProcessed) / elapsed
				}

				h.display.UpdateProgress(int(completed), hashrate)
				if completed >= uint64(h.numPieces) {
					return
				}
			}
		}
	}()

	wg.Wait()
	close(stopProgress)
	<-progressDone
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
// It handles:
// - reading from multiple files that may span piece boundaries
// - maintaining file positions and readers
// - calculating SHA1 hashes for each piece
// - updating progress through the completedPieces counter
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
	readers := make(map[string]*fileReader)
	defer func() {
		for _, reader := range readers {
			_ = reader.file.Close()
		}
	}()

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceReadOffset := pieceOffset
		pieceLength := h.pieceLengthFor(pieceIndex)
		hasher.Reset()
		remainingPiece := pieceLength
		bytesHashed := int64(0)

		startFile := h.startFileForPiece(pieceIndex)
		for fileIndex := startFile; fileIndex < len(h.files) && remainingPiece > 0; fileIndex++ {
			file := h.files[fileIndex]
			if pieceReadOffset >= file.offset+file.length {
				continue
			}

			readStart := max(int64(0), pieceReadOffset-file.offset)
			readLength := min(file.length-readStart, remainingPiece)
			if readLength <= 0 {
				continue
			}

			reader, ok := readers[file.path]
			if !ok {
				f, err := os.Open(file.path)
				if err != nil {
					return fmt.Errorf("failed to open file %s: %w", file.path, err)
				}
				reader = &fileReader{
					file:     f,
					position: 0,
					length:   file.length,
				}
				readers[file.path] = reader
			}

			if reader.position != readStart {
				if _, err := reader.file.Seek(readStart, io.SeekStart); err != nil {
					return fmt.Errorf("failed to seek in file %s: %w", file.path, err)
				}
				reader.position = readStart
			}

			remaining := readLength
			for remaining > 0 {
				n := int(remaining)
				if n > len(buf) {
					n = len(buf)
				}

				read, err := io.ReadFull(reader.file, buf[:n])
				if err != nil && err != io.ErrUnexpectedEOF {
					return fmt.Errorf("failed to read file %s: %w", file.path, err)
				}
				if read == 0 {
					return fmt.Errorf("short read while hashing file %s", file.path)
				}

				hasher.Write(buf[:read])
				remaining -= int64(read)
				remainingPiece -= int64(read)
				pieceReadOffset += int64(read)
				reader.position += int64(read)
				bytesHashed += int64(read)
			}
		}

		if remainingPiece != 0 {
			return fmt.Errorf("failed to hash piece %d completely: %d bytes remaining", pieceIndex, remainingPiece)
		}

		if bytesHashed > 0 {
			atomic.AddInt64(&h.bytesProcessed, bytesHashed)
		}

		h.pieces[pieceIndex] = hasher.Sum(h.pieces[pieceIndex][:0])
		atomic.AddUint64(completedPieces, 1)
	}

	return nil
}

func (h *pieceHasher) pieceLengthFor(pieceIndex int) int64 {
	if pieceIndex == h.numPieces-1 {
		return h.lastPieceLength
	}
	return h.pieceLen
}

func (h *pieceHasher) startFileForPiece(pieceIndex int) int {
	if pieceIndex < 0 || pieceIndex >= len(h.pieceStartFiles) {
		return len(h.files)
	}
	return h.pieceStartFiles[pieceIndex]
}

func buildPieceLayout(files []fileEntry, pieceLen int64, numPieces int) (int64, int64, []int) {
	var totalSize int64
	for _, file := range files {
		totalSize += file.length
	}

	if numPieces == 0 {
		return totalSize, 0, nil
	}

	lastPieceLength := totalSize - (int64(numPieces-1) * pieceLen)
	switch {
	case lastPieceLength < 0:
		lastPieceLength = 0
	case lastPieceLength > pieceLen:
		lastPieceLength = pieceLen
	}

	pieceStartFiles := make([]int, numPieces)
	fileIndex := 0
	for pieceIndex := 0; pieceIndex < numPieces; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * pieceLen
		for fileIndex < len(files) && pieceOffset >= files[fileIndex].offset+files[fileIndex].length {
			fileIndex++
		}
		pieceStartFiles[pieceIndex] = fileIndex
	}

	return totalSize, lastPieceLength, pieceStartFiles
}

func NewPieceHasher(files []fileEntry, pieceLen int64, numPieces int, display Displayer, failOnSeasonPackWarning bool) *pieceHasher {
	totalSize, lastPieceLength, pieceStartFiles := buildPieceLayout(files, pieceLen, numPieces)
	pieceHashStorage := make([]byte, numPieces*sha1.Size)
	pieces := make([][]byte, numPieces)
	for i := range pieces {
		start := i * sha1.Size
		pieces[i] = pieceHashStorage[start : start+sha1.Size : start+sha1.Size]
	}

	return &pieceHasher{
		pieces:                  pieces,
		pieceHashStorage:        pieceHashStorage,
		pieceLen:                pieceLen,
		numPieces:               numPieces,
		files:                   files,
		display:                 display,
		totalSize:               totalSize,
		lastPieceLength:         lastPieceLength,
		pieceStartFiles:         pieceStartFiles,
		failOnSeasonPackWarning: failOnSeasonPackWarning,
	}
}
