package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

// VerifyOptions holds options for the verification process
type VerifyOptions struct {
	TorrentPath string
	ContentPath string
	Verbose     bool
	Quiet       bool
}

type pieceVerifier struct {
	torrentInfo *metainfo.Info
	contentPath string
	pieceLen    int64
	numPieces   int
	files       []fileEntry // Mapped files based on contentPath
	display     *Display    // Changed to concrete type
	bufferPool  *sync.Pool
	readSize    int

	goodPieces    uint64
	badPieces     uint64
	missingPieces uint64 // Pieces belonging to missing files

	badPieceIndices []int
	missingFiles    []string

	bytesVerified int64
	startTime     time.Time
	lastUpdate    time.Time
	mutex         sync.RWMutex
}

// VerifyData checks the integrity of content files against a torrent file.
func VerifyData(opts VerifyOptions) (*VerificationResult, error) {
	mi, err := metainfo.LoadFromFile(opts.TorrentPath)
	if err != nil {
		return nil, fmt.Errorf("could not load torrent file %q: %w", opts.TorrentPath, err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal info dictionary from %q: %w", opts.TorrentPath, err)
	}

	mappedFiles := make([]fileEntry, 0)
	var totalSize int64
	var missingFiles []string
	baseContentPath := filepath.Clean(opts.ContentPath)

	if info.IsDir() {
		// Multi-file torrent
		expectedFiles := make(map[string]int64) // Map relative path to expected size
		for _, f := range info.Files {
			relPath := filepath.Join(f.Path...)
			expectedFiles[relPath] = f.Length
		}

		// Walk the content directory provided by the user
		err = filepath.Walk(baseContentPath, func(currentPath string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				// Handle potential errors during walk (e.g., permission denied)
				// We might want to report this, but for now, let's try to continue
				fmt.Fprintf(os.Stderr, "Warning: error walking path %q: %v\n", currentPath, walkErr)
				return nil // Continue walking if possible
			}
			if fileInfo.IsDir() {
				// If the base path itself is walked, ignore it
				if currentPath == baseContentPath {
					return nil
				}
				// We don't need to process directories further here, only files
				return nil
			}

			relPath, err := filepath.Rel(baseContentPath, currentPath)
			if err != nil {
				// Should not happen if currentPath is within baseContentPath
				return fmt.Errorf("failed to get relative path for %q: %w", currentPath, err)
			}
			relPath = filepath.ToSlash(relPath) // Ensure consistent slashes

			// Check if this file is expected by the torrent
			if expectedSize, ok := expectedFiles[relPath]; ok {
				if fileInfo.Size() != expectedSize {
					// File exists but size mismatch - treat as missing for verification?
					// Or should we try verifying the parts we have? For now, treat as missing.
					missingFiles = append(missingFiles, relPath+" (size mismatch)")
					delete(expectedFiles, relPath) // Remove from expected map
					return nil
				}

				// File exists and size matches
				mappedFiles = append(mappedFiles, fileEntry{
					path:   currentPath, // Use the actual path on disk
					length: fileInfo.Size(),
					offset: totalSize, // Offset relative to the start of torrent data
				})
				totalSize += fileInfo.Size()
				delete(expectedFiles, relPath) // Remove from expected map
			}
			// If the file is not in expectedFiles, it's an extra file, ignore it.

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking content path %q: %w", baseContentPath, err)
		}

		// Any files remaining in expectedFiles are missing
		for relPath := range expectedFiles {
			missingFiles = append(missingFiles, relPath)
		}

	} else {
		// Single-file torrent
		contentFileInfo, err := os.Stat(baseContentPath)
		if err != nil {
			if os.IsNotExist(err) {
				missingFiles = append(missingFiles, info.Name)
			} else {
				return nil, fmt.Errorf("could not stat content file %q: %w", baseContentPath, err)
			}
		} else {
			if contentFileInfo.IsDir() {
				// User provided a directory for a single-file torrent
				// Check if the file exists inside that directory
				filePathInDir := filepath.Join(baseContentPath, info.Name)
				contentFileInfo, err = os.Stat(filePathInDir)
				if err != nil {
					if os.IsNotExist(err) {
						missingFiles = append(missingFiles, info.Name)
					} else {
						return nil, fmt.Errorf("could not stat content file %q: %w", filePathInDir, err)
					}
				} else if contentFileInfo.IsDir() {
					return nil, fmt.Errorf("expected content file %q, but found a directory", filePathInDir)
				} else if contentFileInfo.Size() != info.Length {
					missingFiles = append(missingFiles, info.Name+" (size mismatch)")
				} else {
					// File found inside the directory
					mappedFiles = append(mappedFiles, fileEntry{
						path:   filePathInDir,
						length: contentFileInfo.Size(),
						offset: 0,
					})
					totalSize = contentFileInfo.Size()
				}
			} else {
				// User provided a file path directly
				if contentFileInfo.Size() != info.Length {
					missingFiles = append(missingFiles, info.Name+" (size mismatch)")
				} else {
					mappedFiles = append(mappedFiles, fileEntry{
						path:   baseContentPath,
						length: contentFileInfo.Size(),
						offset: 0,
					})
					totalSize = contentFileInfo.Size()
				}
			}
		}
	}

	// Sort mapped files for consistent processing order (important for piece hashing)
	sort.Slice(mappedFiles, func(i, j int) bool {
		// Determine the original order from the torrent info
		pathI := strings.Join(info.Files[i].Path, "/")
		pathJ := strings.Join(info.Files[j].Path, "/")
		return pathI < pathJ
	})
	// Recalculate offsets after sorting
	currentOffset := int64(0)
	for i := range mappedFiles {
		mappedFiles[i].offset = currentOffset
		currentOffset += mappedFiles[i].length
	}

	// 4. Initialize Verifier
	numPieces := len(info.Pieces) / 20
	verifier := &pieceVerifier{
		torrentInfo:  &info,
		contentPath:  opts.ContentPath,
		pieceLen:     info.PieceLength,
		numPieces:    numPieces,
		files:        mappedFiles,
		display:      NewDisplay(NewFormatter(opts.Verbose)),
		missingFiles: missingFiles,
	}
	verifier.display.SetQuiet(opts.Quiet)

	// 5. Perform Verification (Hashing and Comparison)
	err = verifier.verifyPieces()
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// 6. Compile and Return Results
	completion := 0.0
	if verifier.numPieces > 0 {
		completion = (float64(verifier.goodPieces) / float64(verifier.numPieces)) * 100.0
	}

	result := &VerificationResult{
		TotalPieces:     verifier.numPieces,
		GoodPieces:      int(verifier.goodPieces),
		BadPieces:       int(verifier.badPieces),
		MissingPieces:   int(verifier.missingPieces), // TODO: Calculate this based on missing files
		Completion:      completion,
		BadPieceIndices: verifier.badPieceIndices,
		MissingFiles:    verifier.missingFiles,
	}

	// TODO: Calculate MissingPieces accurately based on which pieces fall within missing file ranges.

	return result, nil
}

// optimizeForWorkload determines optimal read buffer size and number of worker goroutines
// (Similar to hasher, might need adjustments for verification context)
func (v *pieceVerifier) optimizeForWorkload() (int, int) {
	if len(v.files) == 0 {
		return 0, 0
	}

	var totalSize int64
	for _, f := range v.files {
		totalSize += f.length
	}
	avgFileSize := int64(0)
	if len(v.files) > 0 {
		avgFileSize = totalSize / int64(len(v.files))
	}

	var readSize, numWorkers int

	switch {
	case len(v.files) == 1:
		if totalSize < 1<<20 {
			readSize = 64 << 10
			numWorkers = 1
		} else if totalSize < 1<<30 {
			readSize = 4 << 20
			numWorkers = runtime.NumCPU()
		} else {
			readSize = 8 << 20
			numWorkers = runtime.NumCPU() * 2
		}
	case avgFileSize < 1<<20:
		readSize = 256 << 10
		numWorkers = runtime.NumCPU()
	case avgFileSize < 10<<20:
		readSize = 1 << 20
		numWorkers = runtime.NumCPU()
	case avgFileSize < 1<<30:
		readSize = 4 << 20
		numWorkers = runtime.NumCPU() * 2
	default:
		readSize = 8 << 20
		numWorkers = runtime.NumCPU() * 2
	}

	if numWorkers > v.numPieces {
		numWorkers = v.numPieces
	}
	// Ensure at least one worker if there are pieces
	if v.numPieces > 0 && numWorkers == 0 {
		numWorkers = 1
	}

	return readSize, numWorkers
}

// verifyPieces coordinates the parallel verification of pieces.
func (v *pieceVerifier) verifyPieces() error {
	if v.numPieces == 0 {
		v.display.ShowProgress(0)
		v.display.FinishProgress()
		return nil // Nothing to verify
	}

	var numWorkers int                               // Declare numWorkers
	v.readSize, numWorkers = v.optimizeForWorkload() // Use =

	// Initialize buffer pool
	v.bufferPool = &sync.Pool{
		New: func() interface{} {
			// Allocate slightly larger buffer if readSize is small to reduce pool churn
			allocSize := v.readSize
			if allocSize < 64<<10 {
				allocSize = 64 << 10
			}
			buf := make([]byte, allocSize)
			return buf
		},
	}

	v.mutex.Lock()
	v.startTime = time.Now()
	v.lastUpdate = v.startTime
	v.mutex.Unlock()
	v.bytesVerified = 0

	v.display.ShowFiles(v.files)

	var completedPieces uint64
	piecesPerWorker := (v.numPieces + numWorkers - 1) / numWorkers
	errorsCh := make(chan error, numWorkers)

	if v.numPieces > 0 {
		v.display.ShowProgress(v.numPieces)
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		start := i * piecesPerWorker
		end := start + piecesPerWorker
		if end > v.numPieces {
			end = v.numPieces
		}

		wg.Add(1)
		go func(startPiece, endPiece int) {
			defer wg.Done()
			if err := v.verifyPieceRange(startPiece, endPiece, &completedPieces); err != nil {
				errorsCh <- err
			}
		}(start, end)
	}

	// Progress monitoring goroutine
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			completed := atomic.LoadUint64(&completedPieces)
			if completed >= uint64(v.numPieces) {
				bytesVerified := atomic.LoadInt64(&v.bytesVerified)
				v.mutex.RLock()
				elapsed := time.Since(v.startTime).Seconds()
				v.mutex.RUnlock()
				var rate float64
				if elapsed > 0 {
					rate = float64(bytesVerified) / elapsed
				}
				v.display.UpdateProgress(int(completed), rate)
				return
			}

			bytesVerified := atomic.LoadInt64(&v.bytesVerified)
			v.mutex.RLock()
			elapsed := time.Since(v.startTime).Seconds()
			v.mutex.RUnlock()
			var rate float64
			if elapsed > 0 {
				rate = float64(bytesVerified) / elapsed
			}
			v.display.UpdateProgress(int(completed), rate)
		}
	}()

	wg.Wait()
	close(errorsCh)

	// Check for errors from workers
	for err := range errorsCh {
		if err != nil {
			// Log or handle the first error encountered
			v.display.FinishProgress() // Ensure progress bar is cleaned up
			return err
		}
	}

	v.display.FinishProgress()
	return nil
}

// verifyPieceRange processes and verifies a specific range of pieces.
func (v *pieceVerifier) verifyPieceRange(startPiece, endPiece int, completedPieces *uint64) error {
	buf := v.bufferPool.Get().([]byte)
	defer v.bufferPool.Put(buf) // Return buffer to pool when done

	hasher := sha1.New()
	readers := make(map[string]*fileReader) // Cache file handles
	defer func() {
		for _, r := range readers {
			if r.file != nil {
				r.file.Close()
			}
		}
	}()

	currentFileIndex := 0 // Optimization: track the current file index

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		// Declare hash variables outside the scope potentially jumped over by goto
		var expectedHash []byte
		var actualHash []byte

		pieceOffset := int64(pieceIndex) * v.pieceLen
		pieceEndOffset := pieceOffset + v.pieceLen
		hasher.Reset()
		bytesHashedThisPiece := int64(0)

		// Find the starting file for this piece
		// Optimization: Start search from currentFileIndex
		foundStartFile := false
		for fIdx := currentFileIndex; fIdx < len(v.files); fIdx++ {
			file := v.files[fIdx]
			if pieceOffset < file.offset+file.length {
				currentFileIndex = fIdx // Update current file index
				foundStartFile = true
				break
			}
		}
		if !foundStartFile {
			// This piece starts beyond the last file, should not happen with correct totalSize calculation
			// Or it belongs entirely to missing files. Mark as missing?
			// For now, let's assume an error or handle as bad/missing.
			// If we accurately track missing files earlier, we can mark pieces here.
			atomic.AddUint64(&v.missingPieces, 1) // Placeholder
			atomic.AddUint64(completedPieces, 1)
			continue
		}

		// Iterate through files relevant to this piece
		for fIdx := currentFileIndex; fIdx < len(v.files); fIdx++ {
			file := v.files[fIdx]

			// If this file starts after the current piece ends, we're done with this piece
			if file.offset >= pieceEndOffset {
				break
			}

			// Calculate the intersection of the piece and the file
			readStartInFile := int64(0) // Position within the file to start reading
			if pieceOffset > file.offset {
				readStartInFile = pieceOffset - file.offset
			}

			readEndInFile := file.length // Position within the file to end reading
			if pieceEndOffset < file.offset+file.length {
				readEndInFile = pieceEndOffset - file.offset
			}

			readLength := readEndInFile - readStartInFile
			if readLength <= 0 {
				continue // No overlap or invalid range
			}

			// Get or open file reader
			reader, ok := readers[file.path]
			if !ok {
				f, err := os.OpenFile(file.path, os.O_RDONLY, 0)
				if err != nil {
					// File might be missing or unreadable - treat piece as bad/missing
					// TODO: Need a way to mark pieces from missing files specifically
					atomic.AddUint64(&v.badPieces, 1) // Or missingPieces
					v.mutex.Lock()
					v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
					v.mutex.Unlock()
					goto nextPiece // Skip to the next piece
				}
				reader = &fileReader{file: f, position: -1, length: file.length} // position -1 indicates unknown
				readers[file.path] = reader
			}

			// Seek if necessary
			if reader.position != readStartInFile {
				_, err := reader.file.Seek(readStartInFile, io.SeekStart)
				if err != nil {
					atomic.AddUint64(&v.badPieces, 1)
					v.mutex.Lock()
					v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
					v.mutex.Unlock()
					goto nextPiece // Skip to the next piece
				}
				reader.position = readStartInFile
			}

			// Read and hash the relevant part of the file
			bytesToRead := readLength
			for bytesToRead > 0 {
				readSize := int64(len(buf))
				if bytesToRead < readSize {
					readSize = bytesToRead
				}

				n, err := reader.file.Read(buf[:readSize])
				if err != nil && err != io.EOF {
					atomic.AddUint64(&v.badPieces, 1)
					v.mutex.Lock()
					v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
					v.mutex.Unlock()
					goto nextPiece // Skip to the next piece
				}
				if n == 0 && err == io.EOF {
					break // End of file reached
				}

				hasher.Write(buf[:n])
				bytesHashedThisPiece += int64(n)
				reader.position += int64(n)
				bytesToRead -= int64(n)
				atomic.AddInt64(&v.bytesVerified, int64(n))
			}
			// Update pieceOffset for the next file iteration within the same piece
			pieceOffset += readLength
		}

		// Compare calculated hash with expected hash
		expectedHash = v.torrentInfo.Pieces[pieceIndex*20 : (pieceIndex+1)*20] // Use =
		actualHash = hasher.Sum(nil)                                           // Use =

		if bytes.Equal(actualHash, expectedHash) {
			atomic.AddUint64(&v.goodPieces, 1)
		} else {
			atomic.AddUint64(&v.badPieces, 1)
			v.mutex.Lock()
			v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
			v.mutex.Unlock()
		}

	nextPiece:
		atomic.AddUint64(completedPieces, 1)
	}

	return nil
}
