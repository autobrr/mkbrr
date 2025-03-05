package torrent

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/autobrr/mkbrr/internal/display"
	"github.com/autobrr/mkbrr/internal/seasonpack"
	"github.com/autobrr/mkbrr/internal/types"
)

type pieceHasher struct {
	pieces     [][]byte
	pieceLen   int64
	numPieces  int
	files      []types.EntryFile
	display    display.Displayer
	bufferPool *sync.Pool
	readSize   int

	bytesProcessed int64
	startTime      time.Time
	lastUpdate     time.Time
	mutex          sync.RWMutex
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
		totalSize += f.Length
		if f.Length > maxFileSize {
			maxFileSize = f.Length
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

	h.display.ShowFiles(h.files)

	seasonInfo := seasonpack.AnalyzeSeasonPack(h.files)

	if seasonInfo.IsSeasonPack && seasonInfo.VideoFileCount > 1 {
		if seasonInfo.MaxEpisode > seasonInfo.VideoFileCount {
			seasonInfo.IsSuspicious = true
			h.display.ShowSeasonPackWarnings(seasonInfo)
		}
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
	defer h.bufferPool.Put(buf) //lint:ignore SA6002 intentionally passing non-pointer to sync.Pool

	hasher := sha1.New()
	// track open file handles to avoid reopening the same file
	readers := make(map[string]*types.FileReader)
	defer func() {
		for _, r := range readers {
			r.File.Close()
		}
	}()

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		pieceOffset := int64(pieceIndex) * h.pieceLen
		pieceLength := h.pieceLen

		// handle last piece which may be shorter than others
		if pieceIndex == h.numPieces-1 {
			var totalLength int64
			for _, f := range h.files {
				totalLength += f.Length
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
			if pieceOffset >= file.Offset+file.Length {
				continue
			}
			if remainingPiece <= 0 {
				break
			}

			// calculate read boundaries within the current file
			readStart := pieceOffset - file.Offset
			if readStart < 0 {
				readStart = 0
			}

			readLength := file.Length - readStart
			if readLength > remainingPiece {
				readLength = remainingPiece
			}

			// reuse or create new file reader
			reader, ok := readers[file.Path]
			if !ok {
				f, err := os.OpenFile(file.Path, os.O_RDONLY, 0)
				if err != nil {
					return fmt.Errorf("failed to open file %s: %w", file.Path, err)
				}
				reader = &types.FileReader{
					File:     f,
					Position: 0,
					Length:   file.Length,
				}
				readers[file.Path] = reader
			}

			// ensure correct file position before reading
			if reader.Position != readStart {
				if _, err := reader.File.Seek(readStart, 0); err != nil {
					return fmt.Errorf("failed to seek in file %s: %w", file.Path, err)
				}
				reader.Position = readStart
			}

			// read file data in chunks to avoid large memory allocations
			remaining := readLength
			for remaining > 0 {
				n := int(remaining)
				if n > len(buf) {
					n = len(buf)
				}

				read, err := io.ReadFull(reader.File, buf[:n])
				if err != nil && err != io.ErrUnexpectedEOF {
					return fmt.Errorf("failed to read file %s: %w", file.Path, err)
				}

				hasher.Write(buf[:read])
				remaining -= int64(read)
				remainingPiece -= int64(read)
				reader.Position += int64(read)
				pieceOffset += int64(read)

				atomic.AddInt64(&h.bytesProcessed, int64(read))
			}
		}

		// store piece hash and update progress
		h.pieces[pieceIndex] = hasher.Sum(nil)
		atomic.AddUint64(completedPieces, 1)
	}

	return nil
}

func NewPieceHasher(files []types.EntryFile, pieceLen int64, numPieces int, display display.Displayer) *pieceHasher {
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

// MinInt returns the smaller of two integers
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
