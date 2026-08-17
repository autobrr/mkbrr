package torrent

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
)

// UpdateOptions contains options for updating an existing v1 torrent from content on disk.
type UpdateOptions struct {
	TorrentPath      string
	ContentPath      string
	OutputPath       string
	InPlace          bool
	Force            bool
	Renames          map[string]string
	ExcludePatterns  []string
	IncludePatterns  []string
	Workers          int
	Verbose          bool
	Quiet            bool
	ProgressCallback ProgressCallback
}

// UpdateResult summarizes a torrent content update.
type UpdateResult struct {
	OutputPath   string
	InfoHash     string
	TotalPieces  int
	ReusedPieces int
	HashedPieces int
}

type reuseFile struct {
	path   string
	length int64
	offset int64
}

type pieceReuse struct {
	oldFiles     []reuseFile
	oldPieces    []byte
	pieceLength  int64
	renames      map[string]string
	matchedFiles []int
	reused       int
}

// UpdateTorrent structurally synchronizes an existing v1 torrent with content on disk.
// Same-path, same-length files and explicit renames are assumed to be byte-identical;
// callers must perform a full recreate when file bytes may change without changing size.
func UpdateTorrent(opts UpdateOptions) (*UpdateResult, error) {
	if opts.TorrentPath == "" {
		return nil, fmt.Errorf("torrent path is required")
	}
	if opts.ContentPath == "" {
		return nil, fmt.Errorf("content path is required")
	}
	if opts.InPlace && opts.OutputPath != "" {
		return nil, fmt.Errorf("in-place update and output path are mutually exclusive")
	}

	outputPath := opts.OutputPath
	defaultOutput := !opts.InPlace && outputPath == ""
	switch {
	case opts.InPlace:
		outputPath = opts.TorrentPath
	case defaultOutput:
		outputPath = defaultUpdateOutputPath(opts.TorrentPath)
	}
	if defaultOutput {
		if _, err := os.Lstat(outputPath); err == nil {
			return nil, fmt.Errorf("default output %q already exists; choose an output path or use in-place update", outputPath)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("check default output %q: %w", outputPath, err)
		}
	}
	if err := validateUpdateOutputPath(opts.TorrentPath, opts.ContentPath, outputPath, opts.InPlace); err != nil {
		return nil, err
	}
	writeOutputPath, err := updateOutputWritePath(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output parent: %w", err)
	}

	torrentData, err := os.ReadFile(opts.TorrentPath)
	if err != nil {
		return nil, fmt.Errorf("load torrent: %w", err)
	}
	rootMap, err := decodeTorrentRoot(torrentData)
	if err != nil {
		return nil, fmt.Errorf("decode torrent root: %w", err)
	}
	oldInfoBytes, ok := rootMap["info"]
	if !ok {
		return nil, fmt.Errorf("torrent has no info dictionary")
	}

	var oldInfo metainfo.Info
	if err := bencode.Unmarshal(oldInfoBytes, &oldInfo); err != nil {
		return nil, fmt.Errorf("decode torrent info: %w", err)
	}

	oldInfoMap := make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(oldInfoBytes, &oldInfoMap); err != nil {
		return nil, fmt.Errorf("decode raw torrent info: %w", err)
	}
	_, hasFiles := oldInfoMap["files"]
	_, hasLength := oldInfoMap["length"]
	if hasFiles == hasLength {
		if hasFiles {
			return nil, fmt.Errorf("torrent info must not contain both files and length")
		}
		return nil, fmt.Errorf("torrent info must contain either files or length")
	}
	if _, ok := oldInfoMap["meta version"]; ok {
		return nil, fmt.Errorf("updating v2 or hybrid torrents is not supported")
	}
	if _, ok := oldInfoMap["file tree"]; ok {
		return nil, fmt.Errorf("updating v2 or hybrid torrents is not supported")
	}

	reuse, err := newPieceReuse(oldInfo, opts.Renames)
	if err != nil {
		return nil, err
	}

	generated, err := createTorrent(CreateOptions{
		Path:             opts.ContentPath,
		Name:             oldInfo.Name,
		ExcludePatterns:  opts.ExcludePatterns,
		IncludePatterns:  opts.IncludePatterns,
		Workers:          opts.Workers,
		Verbose:          opts.Verbose,
		Quiet:            opts.Quiet,
		NoDate:           true,
		NoCreator:        true,
		ProgressCallback: opts.ProgressCallback,
	}, createTorrentOptions{
		pieceLengthBytes: oldInfo.PieceLength,
		pieceReuse:       reuse,
	})
	if err != nil {
		return nil, fmt.Errorf("update torrent content: %w", err)
	}

	newInfoMap := make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(generated.InfoBytes, &newInfoMap); err != nil {
		return nil, fmt.Errorf("decode updated torrent info: %w", err)
	}

	if err := preserveMappedFileInfoKeys(oldInfoMap, newInfoMap, reuse.matchedFiles); err != nil {
		return nil, err
	}

	delete(oldInfoMap, "files")
	delete(oldInfoMap, "length")
	if files, ok := newInfoMap["files"]; ok {
		oldInfoMap["files"] = files
	} else if length, ok := newInfoMap["length"]; ok {
		oldInfoMap["length"] = length
	} else {
		return nil, fmt.Errorf("updated torrent has neither files nor length")
	}
	oldInfoMap["pieces"] = newInfoMap["pieces"]

	infoBytes, err := bencode.Marshal(oldInfoMap)
	if err != nil {
		return nil, fmt.Errorf("encode updated torrent info: %w", err)
	}
	rootMap["info"] = infoBytes

	var updatedInfo metainfo.Info
	if err := bencode.Unmarshal(infoBytes, &updatedInfo); err != nil {
		return nil, fmt.Errorf("decode updated torrent info: %w", err)
	}
	totalPieces := len(updatedInfo.Pieces) / 20
	oldTotalPieces := len(oldInfo.Pieces) / 20
	if max(oldTotalPieces, totalPieces) > 1 && reuse.reused == 0 && !opts.Force {
		return nil, fmt.Errorf("refusing update with 0 reusable pieces (existing torrent: %d, updated torrent: %d); verify the content path or use --force", oldTotalPieces, totalPieces)
	}

	currentWriteOutputPath, err := updateOutputWritePath(outputPath)
	if err != nil {
		return nil, fmt.Errorf("re-resolve output parent: %w", err)
	}
	if currentWriteOutputPath != writeOutputPath {
		return nil, fmt.Errorf("output parent changed during update")
	}
	if err := writeTorrentAtomically(rootMap, writeOutputPath); err != nil {
		return nil, err
	}

	return &UpdateResult{
		OutputPath:   outputPath,
		InfoHash:     metainfo.HashBytes(infoBytes).String(),
		TotalPieces:  totalPieces,
		ReusedPieces: reuse.reused,
		HashedPieces: totalPieces - reuse.reused,
	}, nil
}

func decodeTorrentRoot(data []byte) (map[string]bencode.Bytes, error) {
	rootMap := make(map[string]bencode.Bytes)
	err := bencode.Unmarshal(data, &rootMap)
	if err == nil {
		return rootMap, nil
	}

	var trailing bencode.ErrUnusedTrailingBytes
	if !errors.As(err, &trailing) {
		return nil, err
	}
	contentEnd := len(data) - trailing.NumUnusedBytes
	for _, value := range data[contentEnd:] {
		switch value {
		case ' ', '\t', '\r', '\n', 0:
		default:
			return nil, err
		}
	}

	rootMap = make(map[string]bencode.Bytes)
	if err := bencode.Unmarshal(data[:contentEnd], &rootMap); err != nil {
		return nil, err
	}
	return rootMap, nil
}

func defaultUpdateOutputPath(torrentPath string) string {
	extension := filepath.Ext(torrentPath)
	if extension == "" {
		return torrentPath + ".updated.torrent"
	}
	return strings.TrimSuffix(torrentPath, extension) + ".updated" + extension
}

func validateUpdateOutputPath(torrentPath, contentPath, outputPath string, inPlace bool) error {
	outputWritePath, err := updateOutputWritePath(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	if !inPlace {
		torrentWritePath, err := updateOutputWritePath(torrentPath)
		if err != nil {
			return fmt.Errorf("resolve torrent path: %w", err)
		}
		torrentInfo, torrentErr := os.Lstat(torrentPath)
		outputInfo, outputErr := os.Lstat(outputPath)
		sameExistingEntry := torrentErr == nil && outputErr == nil && os.SameFile(torrentInfo, outputInfo)
		if outputWritePath == torrentWritePath || sameExistingEntry {
			return fmt.Errorf("output path refers to the input torrent; use in-place update instead")
		}
	}

	contentInfo, err := os.Stat(contentPath)
	if err != nil {
		return nil
	}
	contentResolved, err := resolvedUpdatePath(contentPath)
	if err != nil {
		return fmt.Errorf("resolve content path: %w", err)
	}
	if !contentInfo.IsDir() {
		outputInfo, outputErr := os.Lstat(outputPath)
		if outputWritePath == contentResolved || outputErr == nil && os.SameFile(contentInfo, outputInfo) {
			return fmt.Errorf("output path must not replace the content file")
		}
		return nil
	}

	insideContent, err := updatePathInsideDirectory(contentResolved, outputWritePath, contentInfo)
	if err != nil {
		return fmt.Errorf("compare output and content paths: %w", err)
	}
	if insideContent {
		if inPlace {
			return fmt.Errorf("input torrent must be outside the content directory for in-place update")
		}
		return fmt.Errorf("output path must not be inside the content directory")
	}
	return nil
}

func updatePathInsideDirectory(directoryPath, candidatePath string, directoryInfo os.FileInfo) (bool, error) {
	relative, err := filepath.Rel(directoryPath, candidatePath)
	if err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return true, nil
	}

	if candidateInfo, statErr := os.Lstat(candidatePath); statErr == nil {
		if candidateInfo.IsDir() && os.SameFile(directoryInfo, candidateInfo) {
			return true, nil
		}
	} else if !os.IsNotExist(statErr) {
		return false, statErr
	}

	for current := filepath.Dir(candidatePath); ; current = filepath.Dir(current) {
		currentInfo, statErr := os.Stat(current)
		if statErr == nil {
			if os.SameFile(directoryInfo, currentInfo) {
				return true, nil
			}
		} else if !os.IsNotExist(statErr) {
			return false, statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}

func resolvedUpdatePath(filePath string) (string, error) {
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)

	current := absolute
	suffix := make([]string, 0, 2)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(suffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, suffix[index])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolute, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

// updateOutputWritePath pins the resolved parent while preserving the final
// path component, so replacing an existing output symlink remains atomic.
func updateOutputWritePath(filePath string) (string, error) {
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	parent, err := resolvedUpdatePath(filepath.Dir(filepath.Clean(absolute)))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

// newPieceReuse validates the original v1 piece layout and prepares normalized file mappings.
func newPieceReuse(info metainfo.Info, renames map[string]string) (*pieceReuse, error) {
	if info.PieceLength <= 0 {
		return nil, fmt.Errorf("torrent has invalid piece length %d", info.PieceLength)
	}
	if len(info.Pieces) == 0 || len(info.Pieces)%20 != 0 {
		return nil, fmt.Errorf("torrent has invalid v1 piece hashes")
	}

	torrentName := normalizeTorrentPath(info.Name)
	if torrentName == "" || strings.Contains(torrentName, "/") {
		return nil, fmt.Errorf("torrent has invalid name %q", info.Name)
	}

	oldFiles := make([]reuseFile, 0, max(1, len(info.Files)))
	var offset int64
	if len(info.Files) == 0 {
		if info.Length <= 0 {
			return nil, fmt.Errorf("torrent contains no v1 file data")
		}
		oldFiles = append(oldFiles, reuseFile{
			path:   torrentName,
			length: info.Length,
		})
		offset = info.Length
	} else {
		for index, file := range info.Files {
			if file.Length < 0 {
				return nil, fmt.Errorf("torrent file %d has invalid negative length %d", index, file.Length)
			}
			if file.Length > maxTorrentDataSize-offset {
				return nil, fmt.Errorf("torrent file lengths exceed %d bytes", maxTorrentDataSize)
			}
			filePath := normalizeTorrentPath(strings.Join(file.Path, "/"))
			if filePath == "" {
				return nil, fmt.Errorf("torrent file %d has an invalid path", index)
			}
			oldFiles = append(oldFiles, reuseFile{
				path:   filePath,
				length: file.Length,
				offset: offset,
			})
			offset += file.Length
		}
	}

	expectedPieces, err := pieceCountForSize(offset, info.PieceLength)
	if err != nil {
		return nil, fmt.Errorf("validate torrent piece layout: %w", err)
	}
	if len(info.Pieces)/20 != expectedPieces {
		return nil, fmt.Errorf("torrent has %d piece hashes, want %d for its content length", len(info.Pieces)/20, expectedPieces)
	}

	normalizedRenames := make(map[string]string, len(renames))
	renameSourceKeys := make(map[string]string, len(renames))
	for oldPath, newPath := range renames {
		normalizedOldPath := normalizeTorrentPath(oldPath)
		normalizedNewPath := normalizeTorrentPath(newPath)
		if normalizedOldPath == "" || normalizedNewPath == "" {
			return nil, fmt.Errorf("rename paths must not be empty")
		}
		if _, exists := normalizedRenames[normalizedOldPath]; exists {
			return nil, fmt.Errorf("duplicate normalized rename source %q", normalizedOldPath)
		}
		sourceKey := pathKey(normalizedOldPath)
		if existing, exists := renameSourceKeys[sourceKey]; exists {
			return nil, fmt.Errorf("rename sources %q and %q are ambiguous after Unicode normalization", existing, normalizedOldPath)
		}
		renameSourceKeys[sourceKey] = normalizedOldPath
		normalizedRenames[normalizedOldPath] = normalizedNewPath
	}

	return &pieceReuse{
		oldFiles:    oldFiles,
		oldPieces:   info.Pieces,
		pieceLength: info.PieceLength,
		renames:     normalizedRenames,
	}, nil
}

// findReusablePieces maps each new piece to an identical old piece hash when its full byte range is unchanged.
func (r *pieceReuse) findReusablePieces(files []fileEntry, baseDir string, inputIsDir bool, pieceLength int64) (map[int][]byte, error) {
	if pieceLength != r.pieceLength {
		return nil, fmt.Errorf("piece length changed from %d to %d", r.pieceLength, pieceLength)
	}

	newFiles, err := describeNewFiles(files, baseDir, inputIsDir)
	if err != nil {
		return nil, err
	}
	mapping, err := r.matchFiles(newFiles)
	if err != nil {
		return nil, err
	}
	r.matchedFiles = mapping

	oldTotal := fileListLength(r.oldFiles)
	newTotal := fileListLength(newFiles)
	totalPieces, err := pieceCountForSize(newTotal, pieceLength)
	if err != nil {
		return nil, fmt.Errorf("validate updated piece layout: %w", err)
	}
	reusable := make(map[int][]byte)
	startFile := 0

	for pieceIndex := range totalPieces {
		newStart := int64(pieceIndex) * pieceLength
		newEnd := newTotal
		if newTotal-newStart > pieceLength {
			newEnd = newStart + pieceLength
		}
		for startFile < len(newFiles) && newFiles[startFile].offset+newFiles[startFile].length <= newStart {
			startFile++
		}

		position := newStart
		fileIndex := startFile
		oldStart := int64(-1)
		expectedOldPosition := int64(-1)
		canReuse := true

		for position < newEnd {
			for fileIndex < len(newFiles) && newFiles[fileIndex].offset+newFiles[fileIndex].length <= position {
				fileIndex++
			}
			if fileIndex >= len(newFiles) || mapping[fileIndex] < 0 {
				canReuse = false
				break
			}

			newFile := newFiles[fileIndex]
			oldFile := r.oldFiles[mapping[fileIndex]]
			segmentEnd := min(newEnd, newFile.offset+newFile.length)
			oldPosition := oldFile.offset + position - newFile.offset
			if oldStart < 0 {
				oldStart = oldPosition
			} else if oldPosition != expectedOldPosition {
				canReuse = false
				break
			}
			expectedOldPosition = oldPosition + segmentEnd - position
			position = segmentEnd
			fileIndex++
		}

		pieceSize := newEnd - newStart
		if !canReuse || position != newEnd || oldStart < 0 || oldStart%pieceLength != 0 {
			continue
		}
		oldEnd := oldTotal
		if oldTotal-oldStart > pieceLength {
			oldEnd = oldStart + pieceLength
		}
		if oldEnd-oldStart != pieceSize {
			continue
		}
		oldPieceIndex := int(oldStart / pieceLength)
		hashOffset := oldPieceIndex * 20
		if hashOffset < 0 || hashOffset+20 > len(r.oldPieces) {
			continue
		}
		reusable[pieceIndex] = r.oldPieces[hashOffset : hashOffset+20]
	}

	r.reused = len(reusable)
	return reusable, nil
}

// describeNewFiles converts filesystem entries into torrent-relative files while retaining stream offsets.
func describeNewFiles(files []fileEntry, baseDir string, inputIsDir bool) ([]reuseFile, error) {
	newFiles := make([]reuseFile, len(files))
	for i, file := range files {
		originalPath := file.sourcePath
		if originalPath == "" {
			originalPath = file.path
		}

		var filePath string
		if inputIsDir {
			relPath, err := filepath.Rel(baseDir, originalPath)
			if err != nil {
				return nil, fmt.Errorf("calculate torrent path for %q: %w", originalPath, err)
			}
			filePath = normalizeTorrentPath(relPath)
		} else {
			filePath = normalizeTorrentPath(filepath.Base(originalPath))
		}
		newFiles[i] = reuseFile{path: filePath, length: file.length, offset: file.offset}
	}
	return newFiles, nil
}

// findUniquePath returns the sole unused exact or Unicode-normalized path match.
func findUniquePath(files []reuseFile, used []bool, filePath string, normalized bool) (int, bool) {
	want := filePath
	if normalized {
		want = pathKey(filePath)
	}

	match := -1
	for index, file := range files {
		if used[index] {
			continue
		}
		candidate := file.path
		if normalized {
			candidate = pathKey(candidate)
		}
		if candidate != want {
			continue
		}
		if match >= 0 {
			return -1, true
		}
		match = index
	}
	return match, false
}

// matchFiles maps files that can safely reuse old bytes; unmatched entries are additions or deletions.
func (r *pieceReuse) matchFiles(newFiles []reuseFile) ([]int, error) {
	mapping := make([]int, len(newFiles))
	for i := range mapping {
		mapping[i] = -1
	}
	oldUsed := make([]bool, len(r.oldFiles))
	newUsed := make([]bool, len(newFiles))

	renameKeys := make([]string, 0, len(r.renames))
	for oldPath := range r.renames {
		renameKeys = append(renameKeys, oldPath)
	}
	sort.Strings(renameKeys)

	pendingRenames := make([]string, 0, len(renameKeys))
	for _, oldPath := range renameKeys {
		newPath := r.renames[oldPath]
		oldIndex, oldAmbiguous := findUniquePath(r.oldFiles, oldUsed, oldPath, false)
		newIndex, newAmbiguous := findUniquePath(newFiles, newUsed, newPath, false)
		if oldAmbiguous || newAmbiguous || oldIndex < 0 || newIndex < 0 {
			pendingRenames = append(pendingRenames, oldPath)
			continue
		}
		if r.oldFiles[oldIndex].length != newFiles[newIndex].length {
			return nil, fmt.Errorf("renamed file %q changed size from %d to %d bytes", oldPath, r.oldFiles[oldIndex].length, newFiles[newIndex].length)
		}
		mapping[newIndex] = oldIndex
		oldUsed[oldIndex] = true
		newUsed[newIndex] = true
	}

	for _, oldPath := range pendingRenames {
		newPath := r.renames[oldPath]
		oldIndex, oldAmbiguous := findUniquePath(r.oldFiles, oldUsed, oldPath, true)
		if oldAmbiguous {
			return nil, fmt.Errorf("rename source %q is ambiguous after Unicode normalization", oldPath)
		}
		if oldIndex < 0 {
			return nil, fmt.Errorf("rename source %q is not an unmatched file in the torrent", oldPath)
		}
		newIndex, newAmbiguous := findUniquePath(newFiles, newUsed, newPath, true)
		if newAmbiguous {
			return nil, fmt.Errorf("rename destination %q is ambiguous after Unicode normalization", newPath)
		}
		if newIndex < 0 {
			return nil, fmt.Errorf("rename destination %q is not an unmatched file in the content", newPath)
		}
		if r.oldFiles[oldIndex].length != newFiles[newIndex].length {
			return nil, fmt.Errorf("renamed file %q changed size from %d to %d bytes", oldPath, r.oldFiles[oldIndex].length, newFiles[newIndex].length)
		}
		mapping[newIndex] = oldIndex
		oldUsed[oldIndex] = true
		newUsed[newIndex] = true
	}

	matchUnchanged := func(normalized bool) {
		for newIndex, newFile := range newFiles {
			if newUsed[newIndex] {
				continue
			}
			uniqueNewIndex, newAmbiguous := findUniquePath(newFiles, newUsed, newFile.path, normalized)
			if newAmbiguous || uniqueNewIndex != newIndex {
				continue
			}
			oldIndex, oldAmbiguous := findUniquePath(r.oldFiles, oldUsed, newFile.path, normalized)
			if oldAmbiguous || oldIndex < 0 || r.oldFiles[oldIndex].length != newFile.length {
				continue
			}
			mapping[newIndex] = oldIndex
			oldUsed[oldIndex] = true
			newUsed[newIndex] = true
		}
	}
	matchUnchanged(false)
	matchUnchanged(true)

	return mapping, nil
}

// preserveMappedFileInfoKeys carries raw custom per-file keys to unchanged or explicitly renamed files.
func preserveMappedFileInfoKeys(oldInfoMap, newInfoMap map[string]bencode.Bytes, mapping []int) error {
	oldValue, oldHasFiles := oldInfoMap["files"]
	newValue, newHasFiles := newInfoMap["files"]
	if !oldHasFiles || !newHasFiles {
		return nil
	}

	var oldFiles []map[string]bencode.Bytes
	if err := bencode.Unmarshal(oldValue, &oldFiles); err != nil {
		return fmt.Errorf("decode existing torrent files metadata: %w", err)
	}
	var newFiles []map[string]bencode.Bytes
	if err := bencode.Unmarshal(newValue, &newFiles); err != nil {
		return fmt.Errorf("decode updated torrent files metadata: %w", err)
	}
	if len(mapping) != len(newFiles) {
		return fmt.Errorf("updated torrent has %d file mappings for %d files", len(mapping), len(newFiles))
	}

	for newIndex, oldIndex := range mapping {
		if oldIndex < 0 {
			continue
		}
		if oldIndex >= len(oldFiles) {
			return fmt.Errorf("mapped old file index %d exceeds %d files", oldIndex, len(oldFiles))
		}
		for key, value := range oldFiles[oldIndex] {
			switch key {
			case "length", "path", "path.utf-8":
				continue
			default:
				newFiles[newIndex][key] = value
			}
		}
	}

	filesBytes, err := bencode.Marshal(newFiles)
	if err != nil {
		return fmt.Errorf("encode updated torrent files metadata: %w", err)
	}
	newInfoMap["files"] = filesBytes
	return nil
}

// fileListLength returns the total concatenated length represented by an ordered file list.
func fileListLength(files []reuseFile) int64 {
	if len(files) == 0 {
		return 0
	}
	last := files[len(files)-1]
	return last.offset + last.length
}

// normalizeTorrentPath accepts only relative path syntax without changing filename bytes.
func normalizeTorrentPath(filePath string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if filePath == "" || strings.HasPrefix(filePath, "/") {
		return ""
	}
	cleaned := path.Clean(filePath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

// writeTorrentAtomically writes a raw root dictionary through a same-directory temporary file.
func writeTorrentAtomically(rootMap map[string]bencode.Bytes, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tempFile, err := os.CreateTemp(dir, ".mkbrr-update-*.torrent")
	if err != nil {
		return fmt.Errorf("create temporary torrent: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	mode := os.FileMode(0o644)
	if existing, statErr := os.Stat(outputPath); statErr == nil {
		mode = existing.Mode().Perm()
	}
	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("set torrent permissions: %w", err)
	}
	if err := bencode.NewEncoder(tempFile).Encode(rootMap); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write updated torrent: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close updated torrent: %w", err)
	}
	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("replace torrent %q: %w", outputPath, err)
	}
	return nil
}
