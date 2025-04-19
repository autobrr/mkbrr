package torrent

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Reusing the helper from hasher_test.go to create test files efficiently.
func createTestFilesFastForVerify(t *testing.T, numFiles int, fileSize, pieceLen int64) (string, []fileEntry, [][]byte) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "verify_test_data")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	// No t.Cleanup here, the caller test function should manage cleanup

	var files []fileEntry
	var expectedHashes [][]byte
	var offset int64

	pattern := make([]byte, pieceLen)
	for i := range pattern {
		pattern[i] = byte((i*11 + 7) % 251) // Slightly different pattern for verify tests
	}

	contentPath := tempDir // Base path for content

	if numFiles == 1 {
		// For single file tests, create the file directly in tempDir
		path := filepath.Join(tempDir, "test_file_single.dat")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		// Write pattern logic (simplified for single file)
		if _, err := f.Write(pattern); err != nil {
			f.Close()
			t.Fatalf("failed to write pattern: %v", err)
		}
		if fileSize > int64(len(pattern)) {
			if _, err := f.Seek(fileSize-int64(len(pattern)), io.SeekStart); err != nil {
				f.Close()
				t.Fatalf("failed to seek: %v", err)
			}
			if _, err := f.Write(pattern); err != nil {
				f.Close()
				t.Fatalf("failed to write pattern: %v", err)
			}
		}
		if err := f.Truncate(fileSize); err != nil {
			f.Close()
			t.Fatalf("failed to truncate file: %v", err)
		}
		f.Close()

		files = append(files, fileEntry{path: path, length: fileSize, offset: 0})
		contentPath = path // For single file, content path is the file itself
		offset += fileSize

		// Calculate expected hashes
		h := sha1.New()
		for pos := int64(0); pos < fileSize; pos += pieceLen {
			h.Reset()
			// Simplified pattern application for single file hash calculation
			if pos == 0 || (pos >= fileSize-pieceLen && fileSize > int64(len(pattern))) {
				h.Write(pattern)
			} else if fileSize <= int64(len(pattern)) && pos == 0 {
				h.Write(pattern[:fileSize]) // Handle file smaller than pattern
			} else {
				h.Write(make([]byte, pieceLen))
			}
			expectedHashes = append(expectedHashes, h.Sum(nil))
		}

	} else {
		// Multi-file tests, create files within a subdirectory
		contentPath = filepath.Join(tempDir, "content_dir")
		if err := os.Mkdir(contentPath, 0755); err != nil {
			t.Fatalf("failed to create content dir: %v", err)
		}

		for i := 0; i < numFiles; i++ {
			// Create subdirs for testing nested structure
			subDir := ""
			if i%2 == 0 && numFiles > 1 { // Add some nesting
				subDir = fmt.Sprintf("subdir_%d", i/2)
				if err := os.Mkdir(filepath.Join(contentPath, subDir), 0755); err != nil {
					t.Fatalf("failed to create sub dir: %v", err)
				}
			}
			path := filepath.Join(contentPath, subDir, fmt.Sprintf("test_file_%d.dat", i))

			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				t.Fatalf("failed to create file: %v", err)
			}

			// Write pattern logic
			if _, err := f.Write(pattern); err != nil {
				f.Close()
				t.Fatalf("failed to write pattern: %v", err)
			}
			if fileSize > int64(len(pattern)) {
				if _, err := f.Seek(fileSize-int64(len(pattern)), io.SeekStart); err != nil {
					f.Close()
					t.Fatalf("failed to seek: %v", err)
				}
				if _, err := f.Write(pattern); err != nil {
					f.Close()
					t.Fatalf("failed to write pattern: %v", err)
				}
			}
			if err := f.Truncate(fileSize); err != nil {
				f.Close()
				t.Fatalf("failed to truncate file: %v", err)
			}
			f.Close()

			files = append(files, fileEntry{path: path, length: fileSize, offset: offset})
			offset += fileSize

			// Calculate expected hashes for this file's contribution
			h := sha1.New()
			for pos := int64(0); pos < fileSize; pos += pieceLen {
				h.Reset()
				if pos == 0 || (pos >= fileSize-pieceLen && fileSize > int64(len(pattern))) {
					h.Write(pattern)
				} else if fileSize <= int64(len(pattern)) && pos == 0 {
					h.Write(pattern[:fileSize])
				} else {
					h.Write(make([]byte, pieceLen))
				}
				// Note: This hash calculation is simplified and assumes pieces align with files.
				// The actual verification logic handles pieces spanning files correctly.
				// We mainly need the files created correctly here.
				// The expectedHashes are less critical for verify_test as we compare against the generated torrent.
				expectedHashes = append(expectedHashes, h.Sum(nil))
			}
		}
	}

	return contentPath, files, expectedHashes // Return base content path
}

func TestVerifyData_PerfectMatch_SingleFile(t *testing.T) {
	fileSize := int64(5 * 1024 * 1024) // 5 MiB
	pieceLenExp := uint(18)            // 256 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (fileSize + pieceLen - 1) / pieceLen

	// 1. Create test content file
	contentPath, _, _ := createTestFilesFastForVerify(t, 1, fileSize, pieceLen)
	tempDir := filepath.Dir(contentPath) // Get the parent temp dir for cleanup
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// 2. Create the corresponding torrent file
	torrentPath := filepath.Join(tempDir, "perfect_match_single.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentPath,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false, // Keep simple for test
		NoCreator:      true,
		NoDate:         true,
	}
	_, err := Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 3. Run verification
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		Verbose:     true, // Enable verbose for more info if needed during debugging
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results
	if result.TotalPieces != int(numPieces) {
		t.Errorf("Expected %d total pieces, got %d", numPieces, result.TotalPieces)
	}
	if result.GoodPieces != int(numPieces) {
		t.Errorf("Expected %d good pieces, got %d", numPieces, result.GoodPieces)
	}
	if result.BadPieces != 0 {
		t.Errorf("Expected 0 bad pieces, got %d", result.BadPieces)
	}
	if result.MissingPieces != 0 {
		t.Errorf("Expected 0 missing pieces, got %d", result.MissingPieces)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("Expected 0 missing files, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	if result.Completion != 100.0 {
		t.Errorf("Expected completion 100.0, got %.2f", result.Completion)
	}
}

func TestVerifyData_PerfectMatch_MultiFile(t *testing.T) {
	numFiles := 5
	fileSize := int64(2 * 1024 * 1024) // 2 MiB per file
	totalSize := int64(numFiles) * fileSize
	pieceLenExp := uint(17) // 128 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (totalSize + pieceLen - 1) / pieceLen

	// 1. Create test content files in a directory
	contentDir, _, _ := createTestFilesFastForVerify(t, numFiles, fileSize, pieceLen)
	tempDir := filepath.Dir(contentDir) // Get the parent temp dir for cleanup
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	// 2. Create the corresponding torrent file for the directory
	torrentPath := filepath.Join(tempDir, "perfect_match_multi.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentDir, // Create torrent from the directory
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false,
		NoCreator:      true,
		NoDate:         true,
	}
	_, err := Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 3. Run verification
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir, // Verify against the directory
		Verbose:     true,
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results
	if result.TotalPieces != int(numPieces) {
		t.Errorf("Expected %d total pieces, got %d", numPieces, result.TotalPieces)
	}
	if result.GoodPieces != int(numPieces) {
		t.Errorf("Expected %d good pieces, got %d", numPieces, result.GoodPieces)
	}
	if result.BadPieces != 0 {
		t.Errorf("Expected 0 bad pieces, got %d", result.BadPieces)
	}
	if result.MissingPieces != 0 {
		t.Errorf("Expected 0 missing pieces, got %d", result.MissingPieces)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("Expected 0 missing files, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	if result.Completion != 100.0 {
		t.Errorf("Expected completion 100.0, got %.2f", result.Completion)
	}
}

func TestVerifyData_CorruptedData(t *testing.T) {
	numFiles := 3
	fileSize := int64(1 * 1024 * 1024) // 1 MiB per file
	totalSize := int64(numFiles) * fileSize
	pieceLenExp := uint(16) // 64 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (totalSize + pieceLen - 1) / pieceLen

	// 1. Create test content & torrent
	contentDir, files, _ := createTestFilesFastForVerify(t, numFiles, fileSize, pieceLen)
	tempDir := filepath.Dir(contentDir)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	torrentPath := filepath.Join(tempDir, "corrupted_data.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentDir,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false, NoCreator: true, NoDate: true,
	}
	_, err := Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 2. Corrupt one of the files (modify the first byte of the second file)
	if len(files) > 1 {
		corruptFilePath := files[1].path // Corrupt the second file
		data, err := os.ReadFile(corruptFilePath)
		if err != nil {
			t.Fatalf("Failed to read file for corruption: %v", err)
		}
		if len(data) > 0 {
			data[0] ^= 0xFF // Flip the bits of the first byte
			err = os.WriteFile(corruptFilePath, data, 0644)
			if err != nil {
				t.Fatalf("Failed to write corrupted file: %v", err)
			}
		} else {
			t.Logf("Skipping corruption as file %s is empty", corruptFilePath)
		}
	} else {
		t.Fatal("Test setup error: Need at least 2 files to corrupt the second one.")
	}

	// 3. Run verification
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Verbose:     true,
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		// Expecting verification to succeed functionally, but report bad pieces
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results - Expecting at least one bad piece
	// The exact number depends on whether the corruption spans piece boundaries
	if result.BadPieces == 0 {
		t.Errorf("Expected at least one bad piece due to corruption, got 0")
	}
	if result.GoodPieces == int(numPieces) {
		t.Errorf("Expected fewer good pieces than total pieces due to corruption, got %d/%d", result.GoodPieces, numPieces)
	}
	if result.Completion == 100.0 {
		t.Errorf("Expected completion less than 100.0 due to corruption, got %.2f", result.Completion)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("Expected 0 missing files, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	// We can't easily predict the exact bad piece index without complex calculation,
	// so we mainly check that BadPieces > 0.
	t.Logf("Verification result: %d/%d good, %d bad, %.2f%% complete", result.GoodPieces, result.TotalPieces, result.BadPieces, result.Completion)

}

func TestVerifyData_MissingFile(t *testing.T) {
	numFiles := 4
	fileSize := int64(1 * 1024 * 1024) // 1 MiB per file
	totalSize := int64(numFiles) * fileSize
	pieceLenExp := uint(17) // 128 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (totalSize + pieceLen - 1) / pieceLen

	// 1. Create test content & torrent
	contentDir, files, _ := createTestFilesFastForVerify(t, numFiles, fileSize, pieceLen)
	tempDir := filepath.Dir(contentDir)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	torrentPath := filepath.Join(tempDir, "missing_file.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentDir,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false, NoCreator: true, NoDate: true,
	}
	_, err := Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 2. Delete one of the files (e.g., the second file)
	var deletedFilePathRel string
	if len(files) > 1 {
		deletedFilePath := files[1].path
		deletedFilePathRel, _ = filepath.Rel(contentDir, deletedFilePath) // Get relative path for comparison
		err = os.Remove(deletedFilePath)
		if err != nil {
			t.Fatalf("Failed to delete test file %s: %v", deletedFilePath, err)
		}
	} else {
		t.Fatal("Test setup error: Need at least 2 files to delete one.")
	}

	// 3. Run verification
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Verbose:     true,
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		// Verification itself shouldn't fail, just report missing file/pieces
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results
	if len(result.MissingFiles) != 1 {
		t.Fatalf("Expected 1 missing file, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	if result.MissingFiles[0] != filepath.ToSlash(deletedFilePathRel) {
		t.Errorf("Expected missing file '%s', got '%s'", deletedFilePathRel, result.MissingFiles[0])
	}
	// Depending on implementation, missing files might contribute to BadPieces or MissingPieces
	// Let's check that completion is not 100% and GoodPieces < TotalPieces
	if result.GoodPieces == int(numPieces) {
		t.Errorf("Expected fewer good pieces than total due to missing file, got %d/%d", result.GoodPieces, numPieces)
	}
	if result.Completion == 100.0 {
		t.Errorf("Expected completion less than 100.0 due to missing file, got %.2f", result.Completion)
	}
	// BadPieces count might vary depending on how many pieces the missing file spanned
	t.Logf("Verification result: %d/%d good, %d bad, %d missing pieces (approx), %d missing files, %.2f%% complete",
		result.GoodPieces, result.TotalPieces, result.BadPieces, result.MissingPieces, len(result.MissingFiles), result.Completion)
}

func TestVerifyData_SizeMismatch(t *testing.T) {
	numFiles := 3
	fileSize := int64(1 * 1024 * 1024) // 1 MiB per file
	totalSize := int64(numFiles) * fileSize
	pieceLenExp := uint(17) // 128 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (totalSize + pieceLen - 1) / pieceLen

	// 1. Create test content & torrent
	contentDir, files, _ := createTestFilesFastForVerify(t, numFiles, fileSize, pieceLen)
	tempDir := filepath.Dir(contentDir)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	torrentPath := filepath.Join(tempDir, "size_mismatch.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentDir,
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false, NoCreator: true, NoDate: true,
	}
	_, err := Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 2. Modify the size of one file (truncate the first file)
	var mismatchedFilePathRel string
	if len(files) > 0 {
		mismatchedFilePath := files[0].path
		mismatchedFilePathRel, _ = filepath.Rel(contentDir, mismatchedFilePath)
		err = os.Truncate(mismatchedFilePath, fileSize/2) // Make it smaller
		if err != nil {
			t.Fatalf("Failed to truncate test file %s: %v", mismatchedFilePath, err)
		}
	} else {
		t.Fatal("Test setup error: Need at least 1 file to modify size.")
	}

	// 3. Run verification
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentDir,
		Verbose:     true,
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		// Verification itself shouldn't fail, just report the mismatch
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results
	if len(result.MissingFiles) != 1 {
		t.Fatalf("Expected 1 missing/mismatched file, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	expectedMismatchString := filepath.ToSlash(mismatchedFilePathRel) + " (size mismatch)"
	if result.MissingFiles[0] != expectedMismatchString {
		t.Errorf("Expected mismatched file '%s', got '%s'", expectedMismatchString, result.MissingFiles[0])
	}
	if result.GoodPieces == int(numPieces) {
		t.Errorf("Expected fewer good pieces than total due to size mismatch, got %d/%d", result.GoodPieces, numPieces)
	}
	if result.Completion == 100.0 {
		t.Errorf("Expected completion less than 100.0 due to size mismatch, got %.2f", result.Completion)
	}
	t.Logf("Verification result: %d/%d good, %d bad, %d missing pieces (approx), %d missing/mismatched files, %.2f%% complete",
		result.GoodPieces, result.TotalPieces, result.BadPieces, result.MissingPieces, len(result.MissingFiles), result.Completion)
}

func TestVerifyData_SingleFileInDir(t *testing.T) {
	fileSize := int64(3 * 1024 * 1024) // 3 MiB
	pieceLenExp := uint(17)            // 128 KiB pieces
	pieceLen := int64(1 << pieceLenExp)
	numPieces := (fileSize + pieceLen - 1) / pieceLen
	singleFileName := "test_file_single.dat" // Explicit name

	// 1. Create a single test content file
	tempDir, err := os.MkdirTemp("", "verify_single_in_dir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	contentFilePath := filepath.Join(tempDir, singleFileName)
	// Use a simplified file creation logic here as createTestFilesFastForVerify is complex for this case
	f, err := os.OpenFile(contentFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	// Write some simple data
	data := make([]byte, fileSize)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatalf("failed to write data: %v", err)
	}
	f.Close()

	// 2. Create the corresponding torrent file for the single file
	torrentPath := filepath.Join(tempDir, "single_in_dir.torrent")
	createOpts := CreateTorrentOptions{
		Path:           contentFilePath, // Create torrent FROM the file path
		Name:           singleFileName,  // Ensure torrent name matches file name
		OutputPath:     torrentPath,
		PieceLengthExp: &pieceLenExp,
		IsPrivate:      false, NoCreator: true, NoDate: true,
	}
	_, err = Create(createOpts)
	if err != nil {
		t.Fatalf("Failed to create test torrent file: %v", err)
	}

	// 3. Run verification, providing the DIRECTORY as ContentPath
	verifyOpts := VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: tempDir, // Verify against the directory containing the file
		Verbose:     true,
	}
	result, err := VerifyData(verifyOpts)
	if err != nil {
		t.Fatalf("VerifyData failed unexpectedly: %v", err)
	}

	// 4. Assert results - Should be a perfect match
	if result.TotalPieces != int(numPieces) {
		t.Errorf("Expected %d total pieces, got %d", numPieces, result.TotalPieces)
	}
	if result.GoodPieces != int(numPieces) {
		t.Errorf("Expected %d good pieces, got %d", numPieces, result.GoodPieces)
	}
	if result.BadPieces != 0 {
		t.Errorf("Expected 0 bad pieces, got %d", result.BadPieces)
	}
	if result.MissingPieces != 0 {
		t.Errorf("Expected 0 missing pieces, got %d", result.MissingPieces)
	}
	if len(result.MissingFiles) != 0 {
		t.Errorf("Expected 0 missing files, got %d: %v", len(result.MissingFiles), result.MissingFiles)
	}
	if result.Completion != 100.0 {
		t.Errorf("Expected completion 100.0, got %.2f", result.Completion)
	}
}

func TestVerifyData_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		numFiles      int
		fileSize      int64 // Use 0 for empty file test
		pieceLenExp   uint
		expectedGood  int // Expected good pieces (might be 0 for empty)
		expectedTotal int // Expected total pieces (might be 0 for empty)
		expectedCompl float64
	}{
		{
			name:          "File Smaller Than One Piece",
			numFiles:      1,
			fileSize:      10 * 1024, // 10 KiB
			pieceLenExp:   16,        // 64 KiB pieces
			expectedGood:  1,         // Should have one piece
			expectedTotal: 1,
			expectedCompl: 100.0,
		},
		{
			name:        "MultiFile With Empty File",
			numFiles:    3,           // file1 (1M), file2 (0), file3 (1M)
			fileSize:    1024 * 1024, // 1 MiB (used for non-empty files)
			pieceLenExp: 17,          // 128 KiB
			// Calculation for expected pieces needs care
			// Total size = 2 MiB. Pieces = ceil(2M / 128K) = ceil(16) = 16
			expectedTotal: 16,
			expectedGood:  16, // Empty file contributes 0 bytes but doesn't make pieces bad
			expectedCompl: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "verify_edge_")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			t.Cleanup(func() { os.RemoveAll(tempDir) })

			contentPath := tempDir
			var createPath string // Path used for CreateTorrent

			if tt.name == "MultiFile With Empty File" {
				contentPath = filepath.Join(tempDir, "multi_empty")
				createPath = contentPath
				if err := os.Mkdir(contentPath, 0755); err != nil {
					t.Fatalf("failed to create content dir: %v", err)
				}
				// File 1 (1 MiB)
				path1 := filepath.Join(contentPath, "file1.dat")
				if err := os.WriteFile(path1, make([]byte, tt.fileSize), 0644); err != nil {
					t.Fatalf("failed to write file1: %v", err)
				}
				// File 2 (Empty)
				path2 := filepath.Join(contentPath, "file2_empty.dat")
				if err := os.WriteFile(path2, []byte{}, 0644); err != nil {
					t.Fatalf("failed to write empty file2: %v", err)
				}
				// File 3 (1 MiB)
				path3 := filepath.Join(contentPath, "file3.dat")
				if err := os.WriteFile(path3, make([]byte, tt.fileSize), 0644); err != nil {
					t.Fatalf("failed to write file3: %v", err)
				}
			} else {
				// Single file cases (Empty or Small)
				singleFileName := "edge_file.dat"
				contentPath = filepath.Join(tempDir, singleFileName)
				createPath = contentPath
				if err := os.WriteFile(contentPath, make([]byte, tt.fileSize), 0644); err != nil {
					t.Fatalf("failed to write edge file: %v", err)
				}
			}

			// Create torrent
			torrentPath := filepath.Join(tempDir, tt.name+".torrent")
			plExp := tt.pieceLenExp
			createOpts := CreateTorrentOptions{
				Path:           createPath,
				OutputPath:     torrentPath,
				PieceLengthExp: &plExp,
				IsPrivate:      false, NoCreator: true, NoDate: true,
			}
			_, err = Create(createOpts)
			if err != nil {
				t.Fatalf("Failed to create test torrent file: %v", err)
			}

			// Run verification
			verifyOpts := VerifyOptions{
				TorrentPath: torrentPath,
				ContentPath: createPath, // Verify against the same path used for creation
				Verbose:     true,
			}
			result, err := VerifyData(verifyOpts)
			if err != nil {
				t.Fatalf("VerifyData failed unexpectedly for %s: %v", tt.name, err)
			}

			// Assert results
			if result.TotalPieces != tt.expectedTotal {
				t.Errorf("%s: Expected %d total pieces, got %d", tt.name, tt.expectedTotal, result.TotalPieces)
			}
			if result.GoodPieces != tt.expectedGood {
				t.Errorf("%s: Expected %d good pieces, got %d", tt.name, tt.expectedGood, result.GoodPieces)
			}
			if result.BadPieces != 0 {
				t.Errorf("%s: Expected 0 bad pieces, got %d", tt.name, result.BadPieces)
			}
			if result.MissingPieces != 0 {
				t.Errorf("%s: Expected 0 missing pieces, got %d", tt.name, result.MissingPieces)
			}
			if len(result.MissingFiles) != 0 {
				t.Errorf("%s: Expected 0 missing files, got %d: %v", tt.name, len(result.MissingFiles), result.MissingFiles)
			}
			if result.Completion != tt.expectedCompl {
				t.Errorf("%s: Expected completion %.2f, got %.2f", tt.name, tt.expectedCompl, result.Completion)
			}
		})
	}
}
