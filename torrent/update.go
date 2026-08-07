package torrent

import (
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

	mi, err := metainfo.LoadFromFile(opts.TorrentPath)
	if err != nil {
		return nil, fmt.Errorf("load torrent: %w", err)
	}

	oldInfo, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("decode torrent info: %w", err)
	}

	oldInfoMap := make(map[string]any)
	if err := bencode.Unmarshal(mi.InfoBytes, &oldInfoMap); err != nil {
		return nil, fmt.Errorf("decode raw torrent info: %w", err)
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

	newInfoMap := make(map[string]any)
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
	mi.InfoBytes = infoBytes

	outputPath := opts.OutputPath
	if outputPath == "" {
		outputPath = opts.TorrentPath
	}
	if err := writeTorrentAtomically(mi, outputPath); err != nil {
		return nil, err
	}

	updatedInfo, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("decode written torrent info: %w", err)
	}
	totalPieces := len(updatedInfo.Pieces) / 20
	return &UpdateResult{
		OutputPath:   outputPath,
		InfoHash:     mi.HashInfoBytes().String(),
		TotalPieces:  totalPieces,
		ReusedPieces: reuse.reused,
		HashedPieces: totalPieces - reuse.reused,
	}, nil
}

// newPieceReuse validates the original v1 piece layout and prepares normalized file mappings.
func newPieceReuse(info metainfo.Info, renames map[string]string) (*pieceReuse, error) {
	if info.PieceLength <= 0 {
		return nil, fmt.Errorf("torrent has invalid piece length %d", info.PieceLength)
	}
	if len(info.Pieces) == 0 || len(info.Pieces)%20 != 0 {
		return nil, fmt.Errorf("torrent has invalid v1 piece hashes")
	}

	oldFiles := make([]reuseFile, 0, max(1, len(info.Files)))
	var offset int64
	if len(info.Files) == 0 {
		if info.Length <= 0 {
			return nil, fmt.Errorf("torrent contains no v1 file data")
		}
		oldFiles = append(oldFiles, reuseFile{
			path:   normalizeTorrentPath(info.Name),
			length: info.Length,
		})
		offset = info.Length
	} else {
		for _, file := range info.Files {
			oldFiles = append(oldFiles, reuseFile{
				path:   normalizeTorrentPath(strings.Join(file.Path, "/")),
				length: file.Length,
				offset: offset,
			})
			offset += file.Length
		}
	}

	expectedPieces := int((offset + info.PieceLength - 1) / info.PieceLength)
	if len(info.Pieces) != expectedPieces*20 {
		return nil, fmt.Errorf("torrent has %d piece hashes, want %d for its content length", len(info.Pieces)/20, expectedPieces)
	}

	normalizedRenames := make(map[string]string, len(renames))
	for oldPath, newPath := range renames {
		normalizedOldPath := normalizeTorrentPath(oldPath)
		if _, exists := normalizedRenames[normalizedOldPath]; exists {
			return nil, fmt.Errorf("duplicate normalized rename source %q", normalizedOldPath)
		}
		normalizedRenames[normalizedOldPath] = normalizeTorrentPath(newPath)
	}

	return &pieceReuse{
		oldFiles:    oldFiles,
		oldPieces:   info.Pieces,
		pieceLength: info.PieceLength,
		renames:     normalizedRenames,
	}, nil
}

// findReusablePieces maps each new piece to an identical old piece hash when its full byte range is unchanged.
func (r *pieceReuse) findReusablePieces(files []fileEntry, originalPaths map[string]string, baseDir string, inputIsDir bool, pieceLength int64) (map[int][]byte, error) {
	if pieceLength != r.pieceLength {
		return nil, fmt.Errorf("piece length changed from %d to %d", r.pieceLength, pieceLength)
	}

	newFiles, err := describeNewFiles(files, originalPaths, baseDir, inputIsDir)
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
	totalPieces := int((newTotal + pieceLength - 1) / pieceLength)
	reusable := make(map[int][]byte)
	startFile := 0

	for pieceIndex := range totalPieces {
		newStart := int64(pieceIndex) * pieceLength
		newEnd := min(newStart+pieceLength, newTotal)
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
		oldEnd := min(oldStart+pieceLength, oldTotal)
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
func describeNewFiles(files []fileEntry, originalPaths map[string]string, baseDir string, inputIsDir bool) ([]reuseFile, error) {
	newFiles := make([]reuseFile, len(files))
	for i, file := range files {
		originalPath := originalPaths[file.path]
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

// matchFiles maps files that can safely reuse old bytes; unmatched entries are additions or deletions.
func (r *pieceReuse) matchFiles(newFiles []reuseFile) ([]int, error) {
	mapping := make([]int, len(newFiles))
	for i := range mapping {
		mapping[i] = -1
	}
	oldUsed := make([]bool, len(r.oldFiles))

	findOld := func(filePath string) int {
		for i, file := range r.oldFiles {
			if !oldUsed[i] && file.path == filePath {
				return i
			}
		}
		return -1
	}
	findNew := func(filePath string) int {
		for i, file := range newFiles {
			if mapping[i] < 0 && file.path == filePath {
				return i
			}
		}
		return -1
	}

	renameKeys := make([]string, 0, len(r.renames))
	for oldPath := range r.renames {
		renameKeys = append(renameKeys, oldPath)
	}
	sort.Strings(renameKeys)
	for _, oldPath := range renameKeys {
		newPath := r.renames[oldPath]
		oldIndex := findOld(oldPath)
		if oldIndex < 0 {
			return nil, fmt.Errorf("rename source %q is not an unmatched file in the torrent", oldPath)
		}
		newIndex := findNew(newPath)
		if newIndex < 0 {
			return nil, fmt.Errorf("rename destination %q is not an unmatched file in the content", newPath)
		}
		if r.oldFiles[oldIndex].length != newFiles[newIndex].length {
			return nil, fmt.Errorf("renamed file %q changed size from %d to %d bytes", oldPath, r.oldFiles[oldIndex].length, newFiles[newIndex].length)
		}
		mapping[newIndex] = oldIndex
		oldUsed[oldIndex] = true
	}

	for newIndex, newFile := range newFiles {
		if mapping[newIndex] >= 0 {
			continue
		}
		oldIndex := findOld(newFile.path)
		if oldIndex < 0 {
			continue
		}
		if r.oldFiles[oldIndex].length != newFile.length {
			continue
		}
		mapping[newIndex] = oldIndex
		oldUsed[oldIndex] = true
	}

	return mapping, nil
}

// preserveMappedFileInfoKeys carries custom per-file keys to unchanged or explicitly renamed files.
func preserveMappedFileInfoKeys(oldInfoMap, newInfoMap map[string]any, mapping []int) error {
	oldValue, oldHasFiles := oldInfoMap["files"]
	newValue, newHasFiles := newInfoMap["files"]
	if !oldHasFiles || !newHasFiles {
		return nil
	}

	oldFiles, ok := oldValue.([]any)
	if !ok {
		return fmt.Errorf("existing torrent files metadata has unexpected type %T", oldValue)
	}
	newFiles, ok := newValue.([]any)
	if !ok {
		return fmt.Errorf("updated torrent files metadata has unexpected type %T", newValue)
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
		oldFile, ok := oldFiles[oldIndex].(map[string]any)
		if !ok {
			return fmt.Errorf("existing torrent file %d metadata has unexpected type %T", oldIndex, oldFiles[oldIndex])
		}
		newFile, ok := newFiles[newIndex].(map[string]any)
		if !ok {
			return fmt.Errorf("updated torrent file %d metadata has unexpected type %T", newIndex, newFiles[newIndex])
		}
		for key, value := range oldFile {
			switch key {
			case "length", "path", "path.utf-8":
				continue
			default:
				newFile[key] = value
			}
		}
	}
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

// normalizeTorrentPath canonicalizes user and metainfo paths to relative forward-slash form.
func normalizeTorrentPath(filePath string) string {
	filePath = strings.ReplaceAll(strings.TrimSpace(filePath), "\\", "/")
	filePath = strings.TrimPrefix(filePath, "./")
	filePath = strings.TrimPrefix(filePath, "/")
	if filePath == "" {
		return ""
	}
	cleaned := path.Clean(filePath)
	if cleaned == "." {
		return ""
	}
	return pathKey(cleaned)
}

// writeTorrentAtomically writes metainfo through a same-directory temporary file before replacing the destination.
func writeTorrentAtomically(mi *metainfo.MetaInfo, outputPath string) error {
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
	if err := mi.Write(tempFile); err != nil {
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
