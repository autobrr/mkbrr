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
	Workers     int // Number of worker goroutines for verification
}

type pieceVerifier struct {
	startTime   time.Time
	lastUpdate  time.Time
	torrentInfo *metainfo.Info
	display     *Display // Changed to concrete type
	bufferPool  *sync.Pool
	contentPath string
	files       []fileEntry // Mapped files based on contentPath

	badPieceIndices []int
	missingFiles    []string
	missingRanges   [][2]int64 // Byte ranges [start, end) of missing/mismatched files

	pieceLen  int64
	numPieces int
	readSize  int

	goodPieces    uint64
	badPieces     uint64
	missingPieces uint64 // Pieces belonging to missing files

	bytesVerified int64
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
		expectedFiles := make(map[string]int64) // Map relative path (using '/') to expected size
		for _, f := range info.Files {
			// Ensure the key uses forward slashes, consistent with torrent format
			relPathKey := filepath.ToSlash(filepath.Join(f.Path...))
			expectedFiles[relPathKey] = f.Length
		}

		// Walk the content directory provided by the user
		err = filepath.Walk(baseContentPath, func(currentPath string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: error walking path %q: %v\n", currentPath, walkErr)
				return nil
			}
			if fileInfo.IsDir() {
				if currentPath == baseContentPath {
					return nil
				}
				return nil
			}

			relPath, err := filepath.Rel(baseContentPath, currentPath)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %q: %w", currentPath, err)
			}
			relPath = filepath.ToSlash(relPath) // Ensure consistent slashes

			if expectedSize, ok := expectedFiles[relPath]; ok {
				if fileInfo.Size() != expectedSize {
					missingFiles = append(missingFiles, relPath+" (size mismatch)")
					delete(expectedFiles, relPath)
					return nil
				}

				mappedFiles = append(mappedFiles, fileEntry{
					path:   currentPath,
					length: fileInfo.Size(),
					offset: totalSize,
				})
				totalSize += fileInfo.Size()
				delete(expectedFiles, relPath)
			}
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking content path %q: %w", baseContentPath, err)
		}

		for relPathKey := range expectedFiles {
			missingFiles = append(missingFiles, relPathKey)
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
					mappedFiles = append(mappedFiles, fileEntry{
						path:   filePathInDir,
						length: contentFileInfo.Size(),
						offset: 0,
					})
					totalSize = contentFileInfo.Size()
				}
			} else {
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

	// Sort mapped files based on original torrent order before recalculating offsets
	if info.IsDir() && len(info.Files) > 0 && len(mappedFiles) > 1 {
		originalOrder := make(map[string]int)
		for i, f := range info.Files {
			originalOrder[filepath.ToSlash(filepath.Join(f.Path...))] = i
		}
		sort.SliceStable(mappedFiles, func(i, j int) bool {
			relPathI, _ := filepath.Rel(baseContentPath, mappedFiles[i].path)
			relPathJ, _ := filepath.Rel(baseContentPath, mappedFiles[j].path)
			return originalOrder[filepath.ToSlash(relPathI)] < originalOrder[filepath.ToSlash(relPathJ)]
		})
	}

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

	// Calculate missing ranges *before* verification starts
	if len(verifier.missingFiles) > 0 {
		missingFileSet := make(map[string]bool)
		for _, mf := range verifier.missingFiles {
			basePath := strings.TrimSuffix(mf, " (size mismatch)")
			missingFileSet[basePath] = true
		}

		currentOffset := int64(0)
		if info.IsDir() {
			for _, f := range info.Files {
				relPath := filepath.ToSlash(filepath.Join(f.Path...))
				fileEndOffset := currentOffset + f.Length
				if missingFileSet[relPath] {
					verifier.missingRanges = append(verifier.missingRanges, [2]int64{currentOffset, fileEndOffset})
				}
				currentOffset = fileEndOffset
			}
		} else if len(verifier.missingFiles) > 0 { // Handle single missing file
			verifier.missingRanges = append(verifier.missingRanges, [2]int64{0, info.Length})
		}
	}

	// 5. Perform Verification (Hashing and Comparison)
	// Pass opts.Workers to verifyPieces
	err = verifier.verifyPieces(opts.Workers) // Pass workers from options
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	// 6. Compile and Return Results
	result := &VerificationResult{
		TotalPieces:     verifier.numPieces,
		GoodPieces:      int(verifier.goodPieces),
		BadPieces:       int(verifier.badPieces),
		MissingPieces:   int(verifier.missingPieces), // This is now correctly counted atomically
		Completion:      0.0,                         // Will be calculated below
		BadPieceIndices: verifier.badPieceIndices,
		MissingFiles:    verifier.missingFiles,
	}

	// Final calculation of completion percentage based on pieces that could be checked
	checkablePieces := result.TotalPieces - result.MissingPieces
	if checkablePieces > 0 {
		// Base completion on pieces that were actually checked (good / checkable)
		result.Completion = (float64(result.GoodPieces) / float64(checkablePieces)) * 100.0
	} else if result.TotalPieces > 0 {
		// All pieces were missing or part of missing files
		result.Completion = 0.0
	} else {
		// 0 total pieces (empty torrent)
		result.Completion = 0.0 // Verification of nothing is 0% complete
	}

	return result, nil
}

// optimizeForWorkload determines optimal read buffer size and number of worker goroutines
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
	if v.numPieces > 0 && numWorkers == 0 {
		numWorkers = 1
	}

	return readSize, numWorkers
}

// verifyPieces coordinates the parallel verification of pieces.
// Accepts numWorkersOverride: if > 0, uses this value; otherwise, optimizes automatically.
func (v *pieceVerifier) verifyPieces(numWorkersOverride int) error {
	if v.numPieces == 0 {
		// Don't show progress for 0 pieces
		return nil
	}

	var numWorkers int
	// Use override if provided, otherwise optimize
	if numWorkersOverride > 0 {
		numWorkers = numWorkersOverride
		// Still need readSize if workers are specified
		v.readSize, _ = v.optimizeForWorkload() // Only need readSize
		// Ensure specified workers don't exceed pieces or minimum of 1
		if numWorkers > v.numPieces {
			numWorkers = v.numPieces
		}
		if v.numPieces > 0 && numWorkers <= 0 { // Safety check
			numWorkers = 1
		}
	} else {
		v.readSize, numWorkers = v.optimizeForWorkload() // Optimize both
	}

	// Final safeguard: Ensure at least one worker if there are pieces
	if v.numPieces > 0 && numWorkers <= 0 {
		numWorkers = 1
	}

	v.bufferPool = &sync.Pool{
		New: func() interface{} {
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

	v.display.ShowFiles(v.files, numWorkers)

	var completedPieces uint64
	piecesPerWorker := (v.numPieces + numWorkers - 1) / numWorkers
	errorsCh := make(chan error, numWorkers)

	v.display.ShowProgress(v.numPieces) // Show progress bar only if numPieces > 0

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
			// Update display
			// We might need to adjust UpdateProgress or pass different values
			// For now, let's pass the overall completed count (good+bad+missing)
			v.mutex.RLock()
			elapsed := time.Since(v.startTime).Seconds()
			v.mutex.RUnlock()
			var rate float64
			if elapsed > 0 {
				bytesVerified := atomic.LoadInt64(&v.bytesVerified)
				rate = float64(bytesVerified) / elapsed
			}
			// Pass total completed count and rate to UpdateProgress
			// Note: UpdateProgress might need adjustment if it expects percentage instead of count
			v.display.UpdateProgress(int(completed), rate)

			if completed >= uint64(v.numPieces) {
				return // Exit goroutine when all pieces are processed
			}
		}
	}()

	wg.Wait()
	close(errorsCh)

	for err := range errorsCh {
		if err != nil {
			v.display.FinishProgress()
			return err
		}
	}

	v.display.FinishProgress()
	return nil
}

// verifyPieceRange processes and verifies a specific range of pieces.
func (v *pieceVerifier) verifyPieceRange(startPiece, endPiece int, completedPieces *uint64) error {
	buf := v.bufferPool.Get().([]byte)
	defer v.bufferPool.Put(buf)

	hasher := sha1.New()
	readers := make(map[string]*fileReader)
	defer func() {
		for _, r := range readers {
			if r.file != nil {
				r.file.Close()
			}
		}
	}()

	currentFileIndex := 0

	for pieceIndex := startPiece; pieceIndex < endPiece; pieceIndex++ {
		var expectedHash []byte
		var actualHash []byte

		pieceOffset := int64(pieceIndex) * v.pieceLen
		pieceEndOffset := pieceOffset + v.pieceLen

		// Check if this piece falls within a known missing range
		isMissing := false
		for _, r := range v.missingRanges {
			if pieceOffset < r[1] && pieceEndOffset > r[0] {
				isMissing = true
				break
			}
		}

		if isMissing {
			atomic.AddUint64(&v.missingPieces, 1)
			atomic.AddUint64(completedPieces, 1)
			continue // Skip hashing/comparison for missing pieces
		}

		// If not missing, proceed to hash and compare
		hasher.Reset()
		bytesHashedThisPiece := int64(0)

		foundStartFile := false
		for fIdx := currentFileIndex; fIdx < len(v.files); fIdx++ {
			file := v.files[fIdx]
			if pieceOffset < file.offset+file.length {
				currentFileIndex = fIdx
				foundStartFile = true
				break
			}
		}
		if !foundStartFile {
			// Should not happen if missingRanges logic is correct and piece is not missing
			atomic.AddUint64(&v.badPieces, 1)
			v.mutex.Lock()
			v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
			v.mutex.Unlock()
			atomic.AddUint64(completedPieces, 1)
			continue
		}

		for fIdx := currentFileIndex; fIdx < len(v.files); fIdx++ {
			file := v.files[fIdx]
			if file.offset >= pieceEndOffset {
				break
			}

			readStartInFile := int64(0)
			if pieceOffset > file.offset {
				readStartInFile = pieceOffset - file.offset
			}
			readEndInFile := file.length
			if pieceEndOffset < file.offset+file.length {
				readEndInFile = pieceEndOffset - file.offset
			}
			readLength := readEndInFile - readStartInFile
			if readLength <= 0 {
				continue
			}

			reader, ok := readers[file.path]
			if !ok {
				f, err := os.OpenFile(file.path, os.O_RDONLY, 0)
				if err != nil {
					// File became unreadable after initial check? Mark as bad.
					atomic.AddUint64(&v.badPieces, 1)
					v.mutex.Lock()
					v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
					v.mutex.Unlock()
					goto nextPiece // Use goto to ensure completedPieces is incremented
				}
				reader = &fileReader{file: f, position: -1, length: file.length}
				readers[file.path] = reader
			}

			if reader.position != readStartInFile {
				_, err := reader.file.Seek(readStartInFile, io.SeekStart)
				if err != nil {
					atomic.AddUint64(&v.badPieces, 1)
					v.mutex.Lock()
					v.badPieceIndices = append(v.badPieceIndices, pieceIndex)
					v.mutex.Unlock()
					goto nextPiece
				}
				reader.position = readStartInFile
			}

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
					goto nextPiece
				}
				if n == 0 && err == io.EOF {
					break
				}
				hasher.Write(buf[:n])
				bytesHashedThisPiece += int64(n)
				reader.position += int64(n)
				bytesToRead -= int64(n)
				atomic.AddInt64(&v.bytesVerified, int64(n))
			}
			pieceOffset += readLength
		}

		expectedHash = v.torrentInfo.Pieces[pieceIndex*20 : (pieceIndex+1)*20]
		actualHash = hasher.Sum(nil)

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
