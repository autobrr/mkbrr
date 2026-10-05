// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"math/bits"
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
	"github.com/fatih/color"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/trackers"
)

const maxTorrentDataSize = int64(^uint64(0) >> 1)

func pieceCountForSize(totalSize, pieceLength int64) (int, error) {
	if totalSize < 0 {
		return 0, fmt.Errorf("content size must not be negative")
	}
	if pieceLength <= 0 {
		return 0, fmt.Errorf("piece length must be positive")
	}
	pieces := totalSize / pieceLength
	if totalSize%pieceLength != 0 {
		pieces++
	}
	maxInt := int64(^uint(0) >> 1)
	if pieces > maxInt/20 {
		return 0, fmt.Errorf("torrent requires too many pieces: %d", pieces)
	}
	return int(pieces), nil
}

func addTorrentFileSize(totalSize, fileSize int64) (int64, error) {
	if fileSize < 0 {
		return 0, fmt.Errorf("file size must not be negative")
	}
	if fileSize > maxTorrentDataSize-totalSize {
		return 0, fmt.Errorf("total content size exceeds %d bytes", maxTorrentDataSize)
	}
	return totalSize + fileSize, nil
}

// formatPieceSize returns a human readable piece size, using KiB for sizes < 1024 KiB and MiB for larger sizes
func formatPieceSize(exp uint) string {
	size := uint64(1) << (exp - 10) // convert to KiB
	if size >= 1024 {
		return fmt.Sprintf("%d MiB", size/1024)
	}
	return fmt.Sprintf("%d KiB", size)
}

// trackerPieceLengthBounds returns the automatic bounds for a tracker recommendation.
// A tracker with its own PieceSizeRanges table may use 16 KiB-128 MiB. A tracker
// that uses only the default ranges uses 64 KiB-16 MiB. The tracker MaxPieceLength
// and the user max piece length can lower the upper bound. The upper bound is
// never below the lower bound.
func trackerPieceLengthBounds(rules trackers.Rules, maxPieceLength *uint) (uint, uint) {
	minExp := uint(16)
	maxExp := uint(24)

	if len(rules.PieceSizeRanges) > 0 {
		minExp = 14
		maxExp = 27
	}

	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}

	if maxPieceLength != nil {
		maxExp = min(maxExp, *maxPieceLength, 27)
	}

	return minExp, max(maxExp, minExp)
}

// sizeLimitPieceLengthCeiling returns the largest piece length exponent that
// the torrent size limit can raise the piece length to.
func sizeLimitPieceLengthCeiling(rules trackers.Rules, maxPieceLength *uint) uint {
	// a tracker cap is a hard ceiling; the user max can only lower it
	if rules.MaxPieceLength > 0 {
		if maxPieceLength != nil {
			return min(*maxPieceLength, rules.MaxPieceLength)
		}
		return rules.MaxPieceLength
	}
	if maxPieceLength != nil {
		return min(*maxPieceLength, 27)
	}
	// a custom table without a cap may grow past its largest entry, as the
	// Portugas rules ask ("32+ MiB")
	if len(rules.PieceSizeRanges) > 0 {
		return 27
	}
	return 24
}

// Notice is a message from the piece length choice. The caller decides
// whether to show it.
type Notice struct {
	Warn bool   `json:"warn"`
	Text string `json:"text"`
}

// choosePieceLength picks the piece length exponent from the create settings,
// the content size, and the tracker rules. It covers the automatic choice, the
// target piece count, the bounds for an explicit piece length, the max piece
// length, and the tracker's maximum .torrent size. torrentSize returns the
// .torrent size for an exponent; it is used only when the rules set a size limit.
func choosePieceLength(totalSize int64, opts CreateOptions, rules trackers.Rules, torrentSize func(exp uint) (uint64, error)) (uint, []Notice, error) {
	if opts.PieceLengthExp != nil && opts.TargetPieceCount != nil {
		return 0, nil, fmt.Errorf("cannot use both piece length and target piece count; use one or the other")
	}
	if opts.TargetPieceCount != nil && opts.PieceLengthExp == nil && *opts.TargetPieceCount == 0 {
		return 0, nil, fmt.Errorf("target piece count must be greater than zero")
	}

	var (
		exp     uint
		notices []Notice
	)
	maxExp := uint(27) // absolute max 128 MiB
	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}
	if opts.PieceLengthExp != nil {
		exp = *opts.PieceLengthExp

		// allow every size the tracker can recommend, such as 16 KiB from its own table
		minExp, _ := trackerPieceLengthBounds(rules, nil)
		if exp < minExp || exp > maxExp {
			where := ""
			if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
				where = " for " + opts.TrackerURLs[0]
			}
			return 0, nil, fmt.Errorf("piece length exponent must be between %d (%s) and %d (%s)%s, got: %d",
				minExp, formatPieceSize(minExp), maxExp, formatPieceSize(maxExp), where, exp)
		}

		// A tracker recommendation never rejects an explicit piece length.
		if rec, ok := rules.PieceSizeExp(uint64(totalSize)); ok {
			notices = append(notices, Notice{Text: fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
				totalSize>>20, formatPieceSize(rec))})
			if exp != rec {
				notices = append(notices, Notice{Warn: true, Text: fmt.Sprintf("custom piece length %s differs from recommendation",
					formatPieceSize(exp))})
			}
		}
	} else {
		if opts.MaxPieceLength != nil {
			if *opts.MaxPieceLength < 14 || *opts.MaxPieceLength > maxExp {
				return 0, nil, fmt.Errorf("max piece length exponent must be between 14 (16 KiB) and %d (%d MiB), got: %d",
					maxExp, 1<<(maxExp-20), *opts.MaxPieceLength)
			}
		}

		var notice *Notice
		if opts.TargetPieceCount != nil {
			exp, notice = pieceLengthFromTarget(totalSize, *opts.TargetPieceCount, opts.MaxPieceLength, rules)
		} else {
			exp, notice = automaticPieceLength(totalSize, opts.MaxPieceLength, rules)
		}
		if notice != nil {
			notices = append(notices, *notice)
		}
	}

	if rules.MaxTorrentSize == 0 || torrentSize == nil {
		return exp, notices, nil
	}

	ceiling := sizeLimitPieceLengthCeiling(rules, opts.MaxPieceLength)
	start := exp
	for {
		size, err := torrentSize(exp)
		if err != nil {
			return 0, nil, err
		}
		if size <= rules.MaxTorrentSize {
			break
		}
		if exp >= ceiling {
			return 0, nil, fmt.Errorf("unable to create torrent under size limit (%.1f KiB) even with maximum piece length",
				float64(rules.MaxTorrentSize)/(1<<10))
		}
		exp++
	}
	if exp != start {
		notices = append(notices, Notice{Warn: true, Text: fmt.Sprintf("raised piece length from %s to %s to fit the %.1f KiB torrent size limit",
			formatPieceSize(start), formatPieceSize(exp), float64(rules.MaxTorrentSize)/(1<<10))})
	}
	return exp, notices, nil
}

// pieceLengthFromTarget derives a piece length exponent from a target piece count.
// The result is clamped to [minExp, maxExp] where maxExp considers tracker and user constraints.
func pieceLengthFromTarget(totalSize int64, targetCount uint, maxPieceLength *uint, rules trackers.Rules) (uint, *Notice) {
	minExp := uint(16) // 64 KiB minimum
	maxExp := uint(24) // default max 16 MiB, same as auto-calc

	// resolve ceiling: tracker hard cap (if any), then user max, then default 24
	trackerCap := rules.MaxPieceLength
	if maxPieceLength != nil {
		userMax := min(*maxPieceLength, 27)
		if trackerCap > 0 {
			// tracker cap is a hard ceiling; user can lower but not exceed it
			maxExp = min(userMax, trackerCap)
		} else {
			// no tracker cap: user max can raise above default 24
			maxExp = userMax
		}
	} else if trackerCap > 0 {
		maxExp = trackerCap
	}

	// ensure maxExp is at least minExp
	maxExp = max(maxExp, minExp)

	// guard: targetCount == 0 or totalSize < targetCount → ratio would be 0
	ratio := uint64(0)
	if targetCount > 0 && totalSize > 0 {
		ratio = uint64(totalSize) / uint64(targetCount)
	}

	var exp uint
	if ratio == 0 {
		exp = minExp
	} else {
		// floor(log2(ratio)) via bit length
		exp = uint(bits.Len64(ratio)) - 1
	}

	clamped := min(max(exp, minExp), maxExp)
	if clamped == exp {
		return clamped, nil
	}
	actualPieces := (uint64(totalSize) + (1 << clamped) - 1) / (1 << clamped)
	return clamped, &Notice{Text: fmt.Sprintf("target piece count %d adjusted: using %s pieces (%d actual pieces) due to constraints",
		targetCount, formatPieceSize(clamped), actualPieces)}
}

// automaticPieceLength calculates the optimal piece length based on total size.
// A tracker recommendation is clamped to trackerPieceLengthBounds. Otherwise the
// result is 64 KiB-16 MiB, and a tracker or user max can change the upper bound.
func automaticPieceLength(totalSize int64, maxPieceLength *uint, rules trackers.Rules) (uint, *Notice) {
	if exp, ok := rules.PieceSizeExp(uint64(totalSize)); ok {
		minExp, maxExp := trackerPieceLengthBounds(rules, maxPieceLength)
		exp = min(max(exp, minExp), maxExp)
		return exp, &Notice{Text: fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
			totalSize>>20, formatPieceSize(exp))}
	}

	minExp := uint(16)
	maxExp := uint(24) // default max 16 MiB for automatic calculation, can be overridden up to 2^27
	if rules.MaxPieceLength > 0 {
		maxExp = rules.MaxPieceLength
	}

	// validate maxPieceLength - if it's below minimum, use minimum
	if maxPieceLength != nil {
		if *maxPieceLength < minExp {
			return minExp, nil
		}
		maxExp = min(*maxPieceLength, 27)
	}

	// default calculation for automatic piece length using shared default ranges
	size := uint64(max(totalSize, 1))

	var exp uint
	for _, r := range trackers.DefaultPieceSizeRanges {
		if size <= r.MaxSize {
			exp = r.PieceExp
			break
		}
	}

	return min(exp, maxExp), nil
}

// GetRecommendedPieceLengthExp returns the effective tracker-specific piece
// length exponent for display, or 0 when the tracker has no recommendation.
// It uses the same choice as create with no overrides.
func GetRecommendedPieceLengthExp(trackerURL string, contentSize uint64) uint {
	rules, _ := trackers.Lookup(trackerURL)
	if _, ok := rules.PieceSizeExp(contentSize); !ok {
		return 0
	}
	exp, _, _ := choosePieceLength(int64(min(contentSize, uint64(maxTorrentDataSize))), CreateOptions{}, rules, nil)
	return exp
}

func (t *Torrent) GetInfo() *metainfo.Info {
	info := &metainfo.Info{}
	_ = bencode.Unmarshal(t.InfoBytes, info)
	return info
}

func generateRandomString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

type createTorrentOptions struct {
	pieceLengthBytes int64
	pieceReuse       *pieceReuse
}

// CreateTorrent creates a new torrent file from the given options.
// Returns a Torrent struct containing the metainfo.
// This is the lower-level function; use Create() for a higher-level interface.
func CreateTorrent(opts CreateOptions) (*Torrent, error) {
	return createTorrent(opts, createTorrentOptions{})
}

type preparedCreate struct {
	files       []fileEntry
	totalSize   int64
	baseDir     string
	inputInfo   os.FileInfo
	pieceLength int64
	exp         uint
	rules       trackers.Rules
	notices     []Notice
	encode      func(pieceLength int64, pieces []byte) (*Torrent, error)
	torrentSize func(exp uint) (uint64, error)
	plan        CreatePlan
}

// prepareCreate builds the exact pre-hash state shared by planning and creation.
func prepareCreate(opts CreateOptions, internalOpts createTorrentOptions) (*preparedCreate, error) {
	path := filepath.ToSlash(opts.Path)
	name := opts.Name
	if name == "" {
		// preserve the folder name even for single-file torrents
		name = filepath.Base(filepath.Clean(path))
	}
	// the name has to resolve on a byte-exact filesystem just like the entries
	// in info.files do: it is the filename for a single-file torrent and the
	// root folder for a multi-file one
	name = nfcPath(filepath.Dir(filepath.Clean(path)), name)

	metadata := metadataSpec{
		Private: setTo(opts.IsPrivate),
	}
	if len(opts.TrackerURLs) > 0 {
		metadata.Trackers = setTo(opts.TrackerURLs)
	}
	if len(opts.WebSeeds) > 0 {
		metadata.WebSeeds = opts.WebSeeds
	}
	if opts.Comment != "" {
		metadata.Comment = setTo(opts.Comment)
	}
	if opts.Source != "" {
		metadata.Source = setTo(opts.Source)
	}
	if !opts.NoCreator {
		metadata.CreatedBy = setTo(createdBy(opts.Version))
	}
	if !opts.NoDate {
		metadata.CreationDate = setTo(time.Now().Unix())
	}
	if opts.Entropy {
		metadata.Entropy = setField
	}

	files := make([]fileEntry, 0, 1)
	var totalSize int64
	var baseDir string

	inputInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("error checking path: %w", err)
	}

	// Walk a directory target rather than the root symlink itself. Nested
	// directory symlinks remain intentionally untraversed to avoid cycles and
	// escaping the selected content tree.
	walkPath := filepath.Clean(path)
	if inputInfo.IsDir() {
		walkPath, err = filepath.EvalSymlinks(walkPath)
		if err != nil {
			return nil, fmt.Errorf("resolve input directory: %w", err)
		}
	}
	matchBasePath := walkPath
	if !inputInfo.IsDir() {
		matchBasePath = filepath.Dir(walkPath)
	}

	err = filepath.Walk(walkPath, func(currentPath string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// check if the error is due to a broken symlink during walk
			// if lstat works but stat fails, it's likely a broken link we might handle later
			if _, lerr := os.Lstat(currentPath); lerr == nil {
				// we can lstat it, maybe it's a broken link we can ignore?
				// for now, let's return the original error to maintain behavior.
				// consider adding verbose logging here if needed.
			}
			return walkErr
		}

		lstatInfo, err := os.Lstat(currentPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not lstat %q: %v\n", currentPath, err)
			return nil
		}

		resolvedPath := currentPath
		resolvedInfo := lstatInfo

		// check if it's a symlink
		if lstatInfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(currentPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not readlink %q: %v\n", currentPath, err)
				return nil
			}
			// if link is relative, resolve it based on the link's directory
			if !filepath.IsAbs(linkTarget) {
				linkTarget = filepath.Join(filepath.Dir(currentPath), linkTarget)
			}
			resolvedPath = filepath.Clean(linkTarget)

			// stat target
			statInfo, err := os.Stat(resolvedPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not stat symlink target %q for link %q: %v\n", resolvedPath, currentPath, err)
				return nil // skip broken link or inaccessible target
			}
			resolvedInfo = statInfo
		}

		// Compute relative path from torrent root for glob matching
		relPath, err := filepath.Rel(matchBasePath, currentPath)
		if err != nil {
			return fmt.Errorf("error calculating relative path for %q: %w", currentPath, err)
		}
		// Handle the root directory case
		if relPath == "." {
			relPath = ""
		}

		if resolvedInfo.IsDir() {
			// Check hardcoded directory ignores (safety net)
			if shouldIgnoreDir(currentPath) || shouldIgnoreDir(resolvedPath) {
				return filepath.SkipDir
			}

			// Check user-defined exclude/include patterns for directories
			if relPath != "" {
				shouldSkip, err := shouldIgnoreEntry(relPath, true, opts.ExcludePatterns, opts.IncludePatterns)
				if err != nil {
					return fmt.Errorf("error processing directory patterns for %q: %w", currentPath, err)
				}
				if shouldSkip {
					return filepath.SkipDir
				}
			}

			if baseDir == "" && currentPath == walkPath {
				baseDir = currentPath
			}
			return nil
		}

		// it's a file (or a link pointing to one)
		shouldIgnore, err := shouldIgnoreEntry(relPath, false, opts.ExcludePatterns, opts.IncludePatterns)
		if err != nil {
			return fmt.Errorf("error processing file patterns for %q: %w", currentPath, err)
		}
		if shouldIgnore {
			return nil
		}

		fileSize := resolvedInfo.Size()
		updatedTotalSize, err := addTorrentFileSize(totalSize, fileSize)
		if err != nil {
			return fmt.Errorf("file %q: %w", currentPath, err)
		}

		// Hash the resolved target while retaining the walked path for torrent metadata.
		files = append(files, fileEntry{
			path:       resolvedPath,
			sourcePath: currentPath,
			length:     fileSize,
			offset:     totalSize,
		})
		totalSize = updatedTotalSize
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking path: %w", err)
	}

	// Sort by torrent-visible paths; resolved targets are not unique for symlink aliases.
	sort.Slice(files, func(i, j int) bool {
		leftPath := files[i].sourcePath
		if leftPath == "" {
			leftPath = files[i].path
		}
		rightPath := files[j].sourcePath
		if rightPath == "" {
			rightPath = files[j].path
		}
		return leftPath < rightPath
	})

	// recalculate offsets based on the sorted file order
	// context: https://github.com/autobrr/mkbrr/issues/64
	var currentOffset int64 = 0
	for i := range files {
		files[i].offset = currentOffset
		currentOffset += files[i].length
	}

	if totalSize == 0 {
		return nil, fmt.Errorf("input path %q contains no files or only empty files, cannot create torrent", path)
	}

	info := metainfo.Info{Name: name}
	if len(files) == 1 && !inputInfo.IsDir() {
		// a single file directly uses the simple format
		info.Length = files[0].length
	} else {
		// a directory uses the folder structure, even for a single file
		info.Files = make([]metainfo.FileInfo, len(files))
		for i, f := range files {
			originalFilepath := f.sourcePath
			if originalFilepath == "" {
				originalFilepath = f.path
			}
			relPath, _ := filepath.Rel(baseDir, originalFilepath)
			relPath = nfcPath(baseDir, relPath)
			pathComponents := strings.Split(filepath.ToSlash(relPath), "/") // Ensure forward slashes
			info.Files[i] = metainfo.FileInfo{
				Path:   pathComponents,
				Length: f.length, // Length comes from resolved file
			}
		}
	}

	// encode returns the torrent for a piece length and its piece hashes
	encode := func(pieceLength int64, pieces []byte) (*Torrent, error) {
		info := info
		info.PieceLength = pieceLength
		info.Pieces = pieces
		infoBytes, err := bencode.Marshal(&info)
		if err != nil {
			return nil, fmt.Errorf("error encoding info: %w", err)
		}
		mi := &metainfo.MetaInfo{InfoBytes: infoBytes}
		if _, err := applyMetadata(mi, metadata); err != nil {
			return nil, err
		}
		return &Torrent{mi}, nil
	}

	// torrentSizeForPieceLength predicts the exact .torrent size before hashing.
	// Encode one placeholder piece, then adjust only the bencoded pieces string
	// length. This keeps planning memory usage constant even for very large inputs.
	torrentSizeForPieceLength := func(pieceLength int64) (uint64, error) {
		numPieces, err := pieceCountForSize(totalSize, pieceLength)
		if err != nil {
			return 0, err
		}
		const placeholderBytes int64 = 20
		t, err := encode(pieceLength, make([]byte, placeholderBytes))
		if err != nil {
			return 0, err
		}
		data, err := bencode.Marshal(t.MetaInfo)
		if err != nil {
			return 0, fmt.Errorf("error marshaling torrent data: %w", err)
		}

		piecesBytes := int64(numPieces) * 20
		digitCount := func(n int64) int64 {
			digits := int64(1)
			for n >= 10 {
				n /= 10
				digits++
			}
			return digits
		}
		delta := piecesBytes - placeholderBytes + digitCount(piecesBytes) - digitCount(placeholderBytes)
		return uint64(int64(len(data)) + delta), nil
	}
	torrentSize := func(exp uint) (uint64, error) {
		return torrentSizeForPieceLength(int64(1) << exp)
	}

	pieceLength := internalOpts.pieceLengthBytes
	var (
		rules   trackers.Rules
		exp     uint
		notices []Notice
	)
	if pieceLength == 0 {
		if len(opts.TrackerURLs) > 0 {
			rules, _ = trackers.Lookup(opts.TrackerURLs[0])
		}
		var err error
		exp, notices, err = choosePieceLength(totalSize, opts, rules, torrentSize)
		if err != nil {
			return nil, err
		}
		pieceLength = int64(1) << exp
	} else if len(opts.TrackerURLs) > 0 {
		rules, _ = trackers.Lookup(opts.TrackerURLs[0])
	}

	predictedTorrentSize, err := torrentSizeForPieceLength(pieceLength)
	if err != nil {
		return nil, err
	}
	return &preparedCreate{
		files:       files,
		totalSize:   totalSize,
		baseDir:     baseDir,
		inputInfo:   inputInfo,
		pieceLength: pieceLength,
		exp:         exp,
		rules:       rules,
		notices:     notices,
		encode:      encode,
		torrentSize: torrentSize,
		plan: CreatePlan{
			ContentSize:          totalSize,
			PieceLengthExponent:  exp,
			PieceLengthBytes:     pieceLength,
			PredictedTorrentSize: predictedTorrentSize,
			TrackerSizeLimit:     rules.MaxTorrentSize,
			Notices:              append([]Notice{}, notices...),
		},
	}, nil
}

// ChoosePieceLength resolves the exact piece length create would use without hashing.
func ChoosePieceLength(opts CreateOptions) (uint, []Notice, error) {
	prepared, err := prepareCreate(opts, createTorrentOptions{})
	if err != nil {
		return 0, nil, err
	}
	return prepared.exp, prepared.notices, nil
}

// PlanCreate returns the exact pre-hash create plan for opts.
func PlanCreate(opts CreateOptions) (*CreatePlan, error) {
	prepared, err := prepareCreate(opts, createTorrentOptions{})
	if err != nil {
		return nil, err
	}
	plan := prepared.plan
	return &plan, nil
}

// createTorrent contains the shared creation pipeline with optional internal hash-reuse controls.
func createTorrent(opts CreateOptions, internalOpts createTorrentOptions) (*Torrent, error) {
	prepared, err := prepareCreate(opts, internalOpts)
	if err != nil {
		return nil, err
	}
	files := prepared.files
	totalSize := prepared.totalSize
	baseDir := prepared.baseDir
	inputInfo := prepared.inputInfo
	pieceLength := prepared.pieceLength
	rules := prepared.rules
	encode := prepared.encode
	if opts.Verbose || opts.InfoOnly {
		display := NewDisplay(NewFormatter(true))
		for _, n := range prepared.notices {
			if n.Warn {
				display.ShowWarning(n.Text)
			} else {
				display.ShowMessage(n.Text)
			}
		}
	}
	numPieces, err := pieceCountForSize(totalSize, pieceLength)
	if err != nil {
		return nil, err
	}

	var display Displayer
	if opts.ProgressCallback != nil {
		// Use callback displayer when progress callback is provided
		display = &callbackDisplayer{callback: opts.ProgressCallback}
	} else {
		// Use default display when no callback is provided
		defaultDisplay := NewDisplay(NewFormatter(opts.Verbose || opts.InfoOnly))
		defaultDisplay.SetQuiet(opts.Quiet || opts.InfoOnly)
		display = defaultDisplay
	}

	hasher := NewPieceHasher(files, pieceLength, numPieces, display, opts.FailOnSeasonPackWarning)
	if internalOpts.pieceReuse != nil {
		reusablePieces, err := internalOpts.pieceReuse.findReusablePieces(files, baseDir, inputInfo.IsDir(), pieceLength)
		if err != nil {
			return nil, err
		}
		hasher.reusablePieces = reusablePieces
	}
	// Pass the specified or default worker count from opts
	if err := hasher.hashPieces(opts.Workers); err != nil {
		return nil, err
	}

	t, err := encode(pieceLength, bytes.Join(hasher.pieces, nil))
	if err != nil {
		return nil, err
	}

	// guard: the pre-hash prediction must match the real size exactly.
	if rules.MaxTorrentSize > 0 {
		data, err := bencode.Marshal(t.MetaInfo)
		if err != nil {
			return nil, fmt.Errorf("error marshaling torrent data: %w", err)
		}
		want := prepared.plan.PredictedTorrentSize
		if uint64(len(data)) != want {
			return nil, fmt.Errorf("torrent size %d bytes does not match the predicted %d bytes", len(data), want)
		}
	}

	return t, nil
}

// Create creates a new torrent file with the given options.
// Returns TorrentInfo containing summary information about the created torrent.
// The torrent file is automatically saved to disk based on the output options.
// This is the main high-level function for torrent creation.
func Create(opts CreateOptions) (*TorrentInfo, error) {
	// validate input path
	if _, err := os.Stat(opts.Path); err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", opts.Path, err)
	}

	baseName := filepath.Base(filepath.Clean(opts.Path))
	if opts.Name == "" {
		opts.Name = baseName
	}

	// set name if not provided
	fileName := opts.Name
	if len(opts.TrackerURLs) == 1 && !opts.SkipPrefix {
		fileName = preset.GetDomainPrefix(opts.TrackerURLs[0]) + "_" + fileName
	}

	if opts.OutputDir != "" {
		opts.OutputPath = filepath.Join(opts.OutputDir, fileName+".torrent")
	} else if opts.OutputPath == "" {
		opts.OutputPath = fileName + ".torrent"
	} else if !strings.HasSuffix(opts.OutputPath, ".torrent") {
		opts.OutputPath = opts.OutputPath + ".torrent"
	}

	if opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("error creating output directory %q: %w", opts.OutputDir, err)
		}
	}

	// create torrent
	t, err := CreateTorrent(opts)
	if err != nil {
		return nil, err
	}

	// create output file
	f, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("error creating output file: %w", err)
	}
	defer f.Close()

	// write torrent file
	if err := t.Write(f); err != nil {
		return nil, fmt.Errorf("error writing torrent file: %w", err)
	}

	// get info for display
	info := t.GetInfo()

	// create torrent info for return
	torrentInfo := &TorrentInfo{
		Path:     opts.OutputPath,
		Size:     info.Length,
		InfoHash: t.MetaInfo.HashInfoBytes().String(),
		Files:    len(info.Files),
		Announce: func() string {
			if len(opts.TrackerURLs) > 0 {
				return opts.TrackerURLs[0]
			}
			return ""
		}(),
	}

	// display info if verbose or info-only
	if opts.Verbose || opts.InfoOnly {
		if opts.InfoOnly {
			prevNoColor := color.NoColor
			color.NoColor = true
			defer func() { color.NoColor = prevNoColor }()
		}

		display := NewDisplay(NewFormatter(opts.Verbose || opts.InfoOnly))
		display.ShowTorrentInfo(t, info)
		//if len(info.Files) > 0 {
		//display.ShowFileTree(info)
		//}
	}

	return torrentInfo, nil
}
