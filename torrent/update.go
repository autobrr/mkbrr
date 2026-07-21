package torrent

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
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
	oldFiles    []reuseFile
	oldPieces   []byte
	pieceLength int64
	renames     map[string]string
	reused      int
}

// UpdateTorrent updates the file list and piece hashes of an existing v1 torrent.
// Existing files are trusted to be unchanged when their path and length match.
// Renamed files are matched explicitly or, when unambiguous, by length.
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
		oldFiles = append(oldFiles, reuseFile{length: info.Length})
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
		normalizedRenames[normalizeTorrentPath(oldPath)] = normalizeTorrentPath(newPath)
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
		filePath := ""
		if inputIsDir {
			originalPath := originalPaths[file.path]
			if originalPath == "" {
				originalPath = file.path
			}
			relPath, err := filepath.Rel(baseDir, originalPath)
			if err != nil {
				return nil, fmt.Errorf("calculate torrent path for %q: %w", originalPath, err)
			}
			filePath = normalizeTorrentPath(filepath.ToSlash(relPath))
		}
		newFiles[i] = reuseFile{path: filePath, length: file.length, offset: file.offset}
	}
	return newFiles, nil
}

// matchFiles pairs every original file with its current path, applying explicit renames before automatic matches.
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
			return nil, fmt.Errorf("existing file %q changed size from %d to %d bytes", newFile.path, r.oldFiles[oldIndex].length, newFile.length)
		}
		mapping[newIndex] = oldIndex
		oldUsed[oldIndex] = true
	}

	oldByLength := make(map[int64][]int)
	newByLength := make(map[int64][]int)
	for oldIndex, oldFile := range r.oldFiles {
		if !oldUsed[oldIndex] {
			oldByLength[oldFile.length] = append(oldByLength[oldFile.length], oldIndex)
		}
	}
	for newIndex, newFile := range newFiles {
		if mapping[newIndex] < 0 {
			newByLength[newFile.length] = append(newByLength[newFile.length], newIndex)
		}
	}
	for length, oldIndices := range oldByLength {
		newIndices := newByLength[length]
		if len(oldIndices) == 1 && len(newIndices) == 1 {
			mapping[newIndices[0]] = oldIndices[0]
			oldUsed[oldIndices[0]] = true
		}
	}

	missing := make([]string, 0)
	for oldIndex, used := range oldUsed {
		if !used {
			missing = append(missing, r.oldFiles[oldIndex].path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("could not match existing file %q; use --rename old=new when a rename is ambiguous", missing[0])
	}

	return mapping, nil
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
	return cleaned
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

	if existing, statErr := os.Stat(outputPath); statErr == nil {
		if err := tempFile.Chmod(existing.Mode().Perm()); err != nil {
			_ = tempFile.Close()
			return fmt.Errorf("preserve torrent permissions: %w", err)
		}
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
