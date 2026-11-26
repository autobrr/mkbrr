// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
		return 0, errors.New("content size must not be negative")
	}
	if pieceLength <= 0 {
		return 0, errors.New("piece length must be positive")
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
		return 0, errors.New("file size must not be negative")
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

// pieceLengthRangeError returns the error for a -l or -m exponent outside [lower, upper].
func pieceLengthRangeError(name string, lower, upper, got uint, trackerURLs []string) error {
	where := ""
	if len(trackerURLs) > 0 && trackerURLs[0] != "" {
		where = " for " + trackerURLs[0]
	}
	return fmt.Errorf("%s exponent must be between %d (%s) and %d (%s)%s, got: %d",
		name, lower, formatPieceSize(lower), upper, formatPieceSize(upper), where, got)
}

// checkExplicitPieceLength returns an error when the explicit piece length,
// or the max piece length if no piece length is set, is outside the allowed
// piece lengths. Create and batch both call it, so they accept the same values.
func checkExplicitPieceLength(pieceLengthExp, maxPieceLength *uint, targetCount bool, rules trackers.Rules, trackerURLs []string) error {
	name, exp := "piece length", pieceLengthExp
	if exp == nil {
		name, exp = "max piece length", maxPieceLength
	}
	if exp == nil {
		return nil
	}
	lower, upper := trackers.AllowedPieceLengths(rules, targetCount)
	if *exp < lower || *exp > upper {
		return pieceLengthRangeError(name, lower, upper, *exp, trackerURLs)
	}
	return nil
}

// Notice is a message from the piece length choice or from modify. The caller
// decides whether to show it.
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
		return 0, nil, errors.New("cannot use both piece length and target piece count; use one or the other")
	}
	if opts.TargetPieceCount != nil && opts.PieceLengthExp == nil && *opts.TargetPieceCount == 0 {
		return 0, nil, errors.New("target piece count must be greater than zero")
	}

	var (
		exp     uint
		notices []Notice
	)
	if err := checkExplicitPieceLength(opts.PieceLengthExp, opts.MaxPieceLength, opts.TargetPieceCount != nil, rules, opts.TrackerURLs); err != nil {
		return 0, nil, err
	}
	lower, upper := trackers.PieceLengthBounds(rules, opts.MaxPieceLength, opts.TargetPieceCount != nil)
	if opts.PieceLengthExp != nil {
		exp = *opts.PieceLengthExp

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
		var notice *Notice
		if opts.TargetPieceCount != nil {
			exp, notice = pieceLengthFromTarget(totalSize, *opts.TargetPieceCount, lower, upper)
		} else {
			exp, notice = automaticPieceLength(totalSize, rules, lower, upper)
		}
		if notice != nil {
			notices = append(notices, *notice)
		}
	}

	if rules.MaxTorrentSize == 0 || torrentSize == nil {
		return exp, notices, nil
	}

	// the retry may grow past the 24 that a target piece count uses
	_, ceiling := trackers.PieceLengthBounds(rules, opts.MaxPieceLength, false)
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
// The result is clamped to [lower, upper].
func pieceLengthFromTarget(totalSize int64, targetCount uint, lower, upper uint) (uint, *Notice) {
	// guard: targetCount == 0 or totalSize < targetCount → ratio would be 0
	ratio := uint64(0)
	if targetCount > 0 && totalSize > 0 {
		ratio = uint64(totalSize) / uint64(targetCount)
	}

	var exp uint
	if ratio == 0 {
		exp = lower
	} else {
		// floor(log2(ratio)) via bit length
		exp = uint(bits.Len64(ratio)) - 1
	}

	clamped := min(max(exp, lower), upper)
	if clamped == exp {
		return clamped, nil
	}
	actualPieces := (uint64(totalSize) + (1 << clamped) - 1) / (1 << clamped)
	return clamped, &Notice{Text: fmt.Sprintf("target piece count %d adjusted: using %s pieces (%d actual pieces) due to constraints",
		targetCount, formatPieceSize(clamped), actualPieces)}
}

// automaticPieceLength calculates the optimal piece length based on total size.
// A tracker recommendation is clamped to [lower, upper]. Otherwise the result
// comes from the default ranges, capped at upper.
func automaticPieceLength(totalSize int64, rules trackers.Rules, lower, upper uint) (uint, *Notice) {
	if exp, ok := rules.PieceSizeExp(uint64(totalSize)); ok {
		exp = min(max(exp, lower), upper)
		return exp, &Notice{Text: fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
			totalSize>>20, formatPieceSize(exp))}
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

	return min(exp, upper), nil
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
	return hex.EncodeToString(b), nil
}

type createTorrentOptions struct {
	pieceLengthBytes int64
	pieceReuse       *pieceReuse
}

// CreateTorrent creates a new torrent file from the given options.
// Returns a Torrent struct containing the metainfo.
// This is the lower-level function; use Create() for a higher-level interface.
func CreateTorrent(opts CreateOptions) (*Torrent, error) {
	return CreateTorrentContext(context.Background(), opts)
}

// CreateTorrentContext is CreateTorrent that stops when ctx is done. On a stop it
// returns a nil torrent and an error that wraps ctx.Err(), after every worker has
// stopped. A read that the operating system blocks continues until it returns.
func CreateTorrentContext(ctx context.Context, opts CreateOptions) (*Torrent, error) {
	t, _, err := createTorrent(ctx, opts, createTorrentOptions{})
	return t, err
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
	seasonInfo  *SeasonPackInfo
	encode      func(pieceLength int64, pieces []byte) (*Torrent, error)
	plan        CreatePlan
}

// prepareCreate builds the exact pre-hash state shared by planning and creation.
func prepareCreate(opts CreateOptions, internalOpts createTorrentOptions) (*preparedCreate, error) {
	return prepareCreateContext(context.Background(), opts, internalOpts)
}

func prepareCreateContext(ctx context.Context, opts CreateOptions, internalOpts createTorrentOptions) (*preparedCreate, error) {
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
		metadata.WebSeeds = setTo(opts.WebSeeds)
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
			if currentPath != walkPath && isMissingTempTorrentFile(currentPath, walkErr) {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		lstatInfo, err := os.Lstat(currentPath)
		if err != nil {
			if isMissingTempTorrentFile(currentPath, err) {
				return nil
			}
			if errors.Is(err, os.ErrNotExist) {
				return err
			}
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

		if isTempTorrentFilename(filepath.Base(currentPath)) {
			if _, err := os.Lstat(currentPath); errors.Is(err, os.ErrNotExist) {
				return nil
			}
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
	var currentOffset int64
	for i := range files {
		files[i].offset = currentOffset
		currentOffset += files[i].length
	}

	if totalSize == 0 {
		return nil, fmt.Errorf("input path %q contains no files or only empty files, cannot create torrent", path)
	}

	seasonInfo := AnalyzeSeasonPack(files)
	if seasonInfo.IsSuspicious && opts.FailOnSeasonPackWarning {
		return nil, fmt.Errorf("season pack is suspicious (season %d, missing episodes %v), and --fail-on-season-warning is enabled", seasonInfo.Season, seasonInfo.MissingEpisodes)
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
		digitCount := func(n int64) int64 { return int64(len(strconv.FormatInt(n, 10))) }
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
		seasonInfo:  seasonInfo,
		encode:      encode,
		plan: CreatePlan{
			ContentSize:          totalSize,
			PieceLengthExponent:  exp,
			PieceLengthBytes:     pieceLength,
			PredictedTorrentSize: predictedTorrentSize,
			TrackerSizeLimit:     rules.MaxTorrentSize,
			Notices:              append([]Notice{}, notices...),
			SeasonPack:           seasonInfo,
		},
	}, nil
}

// ChoosePieceLength chooses the exact piece length create would use without hashing.
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
	plan.OutputPath = createOutputPath(opts)
	return &plan, nil
}

// createTorrent contains the shared creation pipeline with optional internal hash-reuse controls.
// It also returns the season pack check of the files in the torrent.
func createTorrent(ctx context.Context, opts CreateOptions, internalOpts createTorrentOptions) (*Torrent, *SeasonPackInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	prepared, err := prepareCreateContext(ctx, opts, internalOpts)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
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

	display.ShowSeasonPackWarnings(prepared.seasonInfo)
	hasher := newPieceHasher(files, pieceLength, numPieces, display)
	if internalOpts.pieceReuse != nil {
		reusablePieces, err := internalOpts.pieceReuse.findReusablePieces(files, baseDir, inputInfo.IsDir(), pieceLength)
		if err != nil {
			return nil, nil, err
		}
		hasher.reusablePieces = reusablePieces
	}
	if err := hasher.hashPiecesContext(ctx, opts.Workers); err != nil {
		return nil, nil, err
	}

	t, err := encode(pieceLength, bytes.Join(hasher.pieces, nil))
	if err != nil {
		return nil, nil, err
	}

	// guard: the pre-hash prediction must match the real size exactly.
	if rules.MaxTorrentSize > 0 {
		data, err := bencode.Marshal(t.MetaInfo)
		if err != nil {
			return nil, nil, fmt.Errorf("error marshaling torrent data: %w", err)
		}
		want := prepared.plan.PredictedTorrentSize
		if uint64(len(data)) != want {
			return nil, nil, fmt.Errorf("torrent size %d bytes does not match the predicted %d bytes", len(data), want)
		}
	}

	return t, prepared.seasonInfo, nil
}

// Create creates a new torrent file with the given options.
// Returns TorrentInfo containing summary information about the created torrent.
// The torrent file is automatically saved to disk based on the output options.
// This is the main high-level function for torrent creation.
func Create(opts CreateOptions) (*TorrentInfo, error) {
	return CreateContext(context.Background(), opts)
}

// CreateContext is Create that stops when ctx is done, as CreateTorrentContext
// describes. A stop never creates or changes the file at the output path.
func CreateContext(ctx context.Context, opts CreateOptions) (*TorrentInfo, error) {
	t, torrentInfo, err := createRun(ctx, opts)
	if err != nil {
		return nil, err
	}

	if opts.Verbose || opts.InfoOnly {
		if opts.InfoOnly {
			prevNoColor := color.NoColor
			color.NoColor = true
			defer func() { color.NoColor = prevNoColor }()
		}

		display := NewDisplay(NewFormatter(true))
		display.ShowTorrentInfo(t, t.GetInfo())
	}

	return torrentInfo, nil
}

func createRun(ctx context.Context, opts CreateOptions) (*Torrent, *TorrentInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(opts.Path); err != nil {
		return nil, nil, fmt.Errorf("invalid path %q: %w", opts.Path, err)
	}

	outputPath := createOutputPath(opts)

	if err := prepareOutputFolder(outputPath); err != nil {
		return nil, nil, err
	}

	t, seasonInfo, err := createTorrent(ctx, opts, createTorrentOptions{})
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	if err := writeTorrentFile(t, outputPath); err != nil {
		return nil, nil, err
	}

	torrentInfo := newTorrentInfo(t, outputPath, seasonInfo)
	if len(opts.TrackerURLs) > 0 {
		torrentInfo.Announce = opts.TrackerURLs[0]
	}
	return t, torrentInfo, nil
}

var (
	publishCreatedTorrent = publishTorrentFile
	openTempTorrentFile   = os.OpenFile
)

func writeTorrentFile(t *Torrent, outputPath string) error {
	perm := os.FileMode(0o666)
	old, err := os.Lstat(outputPath)
	replacing := err == nil && old.Mode().IsRegular()
	if replacing {
		// Renaming does not require opening the old file for reading and writing.
		probe, err := os.OpenFile(outputPath, os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		_ = probe.Close()
		// Another user who opens a wider temporary file keeps that handle after a chmod.
		perm = old.Mode().Perm()
	}
	f, unregister, err := createTempTorrentFile(outputPath, perm)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer unregister()
	if replacing {
		err = f.Chmod(perm)
	}
	if err == nil {
		err = t.Write(f)
	}
	if err == nil {
		err = f.Sync()
	}
	if err = cmp.Or(err, f.Close()); err == nil {
		err = publishCreatedTorrent(f.Name(), outputPath, true)
	}
	if err != nil {
		return errors.Join(fmt.Errorf("error writing torrent file %q: %w", outputPath, err), os.Remove(f.Name()))
	}
	return nil
}

const (
	tempTorrentFilePrefix = ".mkbrr-"
	tempTorrentFileSuffix = ".tmp"
)

var activeTempTorrents = struct {
	mu    sync.Mutex
	count atomic.Int32
	names map[string]int
}{}

func registerActiveTempTorrent(base string) func() {
	key := strings.ToLower(base)
	activeTempTorrents.mu.Lock()
	if activeTempTorrents.names == nil {
		activeTempTorrents.names = make(map[string]int)
	}
	activeTempTorrents.names[key]++
	activeTempTorrents.count.Add(1)
	activeTempTorrents.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			activeTempTorrents.mu.Lock()
			if activeTempTorrents.names[key] <= 1 {
				delete(activeTempTorrents.names, key)
			} else {
				activeTempTorrents.names[key]--
			}
			activeTempTorrents.count.Add(-1)
			activeTempTorrents.mu.Unlock()
		})
	}
}

func isTempTorrentFilename(base string) bool {
	lower := strings.ToLower(base)
	return strings.HasPrefix(lower, tempTorrentFilePrefix) && strings.HasSuffix(lower, tempTorrentFileSuffix)
}

func isMissingTempTorrentFile(path string, err error) bool {
	return errors.Is(err, os.ErrNotExist) && isTempTorrentFilename(filepath.Base(path))
}

func isActiveTempTorrentFilename(base string) bool {
	if activeTempTorrents.count.Load() == 0 {
		return false
	}
	key := strings.ToLower(base)
	activeTempTorrents.mu.Lock()
	_, ok := activeTempTorrents.names[key]
	activeTempTorrents.mu.Unlock()
	return ok
}

// createTempTorrentFile keeps the parent as given, because a cleaned "link/.." can name another filesystem.
func createTempTorrentFile(outputPath string, perm os.FileMode) (*os.File, func(), error) {
	dir, _ := filepath.Split(outputPath)
	base := tempTorrentFilePrefix + rand.Text() + tempTorrentFileSuffix
	unregister := registerActiveTempTorrent(base)
	f, err := openTempTorrentFile(dir+base, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		unregister()
		return nil, nil, err
	}
	return f, unregister, nil
}

func prepareOutputFolder(outputPath string) error {
	dir, _ := filepath.Split(outputPath)
	folder := outputFolderName(dir)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("error creating output directory %q: %w", folder, err)
		}
	}

	f, unregister, err := createTempTorrentFile(outputPath, 0o600)
	if err != nil {
		return fmt.Errorf("cannot write to output directory %q: %w", folder, err)
	}
	defer unregister()
	tempPath := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(tempPath)
	if err := errors.Join(closeErr, removeErr); err != nil {
		return fmt.Errorf("cannot write to output directory %q: %w", folder, err)
	}
	return nil
}

func outputFolderName(dir string) string {
	if dir == "" {
		return "."
	}
	vol := filepath.VolumeName(dir)
	minLen := len(vol)
	if len(dir) > minLen && os.IsPathSeparator(dir[minLen]) {
		minLen++
	}
	end := len(dir)
	for end > minLen && os.IsPathSeparator(dir[end-1]) {
		end--
	}
	return dir[:end]
}

func joinOutputDir(dir, out string) string {
	if os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + out
	}
	if vol := filepath.VolumeName(dir); vol == dir && len(vol) == 2 && vol[1] == ':' {
		return dir + out
	}
	return dir + string(os.PathSeparator) + out
}

// createOutputPath must not read the disk, so that a plan can give the path without a create.
func createOutputPath(opts CreateOptions) string {
	if opts.OutputPath == "" {
		name := defaultTorrentFilename(opts)
		if opts.OutputDir != "" {
			return filepath.Join(opts.OutputDir, name)
		}
		return name
	}

	out := opts.OutputPath
	if !strings.HasSuffix(out, ".torrent") {
		out += ".torrent"
	}
	if opts.OutputDir != "" && !filepath.IsAbs(out) {
		return joinOutputDir(opts.OutputDir, out)
	}
	return out
}

func defaultTorrentFilename(opts CreateOptions) string {
	name := opts.Name
	if name == "" {
		name = filepath.Base(filepath.Clean(opts.Path))
	}
	if len(opts.TrackerURLs) == 1 && !opts.SkipPrefix {
		name = preset.GetDomainPrefix(opts.TrackerURLs[0]) + "_" + name
	}
	return name + ".torrent"
}

// newTorrentInfo summarizes a created torrent written to path.
func newTorrentInfo(t *Torrent, path string, seasonInfo *SeasonPackInfo) *TorrentInfo {
	info := t.GetInfo()
	return &TorrentInfo{
		Path:       path,
		InfoHash:   t.HashInfoBytes().String(),
		Size:       info.TotalLength(),
		Files:      max(1, len(info.Files)),
		PieceCount: info.NumPieces(),
		SeasonPack: seasonInfo,
	}
}
