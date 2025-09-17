package torrent

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/fatih/color"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/trackers"
)

// max returns the larger of x or y
func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

// formatPieceSize returns a human readable piece size, using KiB for sizes < 1024 KiB and MiB for larger sizes
func formatPieceSize(exp uint) string {
	size := uint64(1) << (exp - 10) // convert to KiB
	if size >= 1024 {
		return fmt.Sprintf("%d MiB", size/1024)
	}
	return fmt.Sprintf("%d KiB", size)
}

// calculatePieceLength calculates the optimal piece length based on total size.
// The min/max bounds (2^16 to 2^24) take precedence over other constraints
func calculatePieceLength(totalSize int64, maxPieceLength *uint, trackerURLs []string, verbose bool) uint {
	minExp := uint(16)
	maxExp := uint(24) // default max 16 MiB for automatic calculation, can be overridden up to 2^27

	// check if tracker has a maximum piece length constraint
	if len(trackerURLs) > 0 && trackerURLs[0] != "" {
		if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(trackerURLs[0]); ok {
			maxExp = trackerMaxExp
		}

		// check if tracker has specific piece size ranges
		if exp, ok := trackers.GetTrackerPieceSizeExp(trackerURLs[0], uint64(totalSize)); ok {
			// ensure we stay within bounds
			if exp < minExp {
				exp = minExp
			}
			if exp > maxExp {
				exp = maxExp
			}
			if verbose {
				display := NewDisplay(NewFormatter(verbose))
				display.ShowMessage(fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
					totalSize>>20, formatPieceSize(exp)))
			}
			return exp
		}
	}

	// validate maxPieceLength - if it's below minimum, use minimum
	if maxPieceLength != nil {
		if *maxPieceLength < minExp {
			return minExp
		}
		if *maxPieceLength > 27 {
			maxExp = 27
		} else {
			maxExp = *maxPieceLength
		}
	}

	// default calculation for automatic piece length
	// ensure minimum of 1 byte for calculation
	size := max(totalSize, 1)

	var exp uint
	switch {
	case size <= 64<<20: // 0 to 64 MB: 32 KiB pieces (2^15)
		exp = 15
	case size <= 128<<20: // 64-128 MB: 64 KiB pieces (2^16)
		exp = 16
	case size <= 256<<20: // 128-256 MB: 128 KiB pieces (2^17)
		exp = 17
	case size <= 512<<20: // 256-512 MB: 256 KiB pieces (2^18)
		exp = 18
	case size <= 1024<<20: // 512 MB-1 GB: 512 KiB pieces (2^19)
		exp = 19
	case size <= 2048<<20: // 1-2 GB: 1 MiB pieces (2^20)
		exp = 20
	case size <= 4096<<20: // 2-4 GB: 2 MiB pieces (2^21)
		exp = 21
	case size <= 8192<<20: // 4-8 GB: 4 MiB pieces (2^22)
		exp = 22
	case size <= 16384<<20: // 8-16 GB: 8 MiB pieces (2^23)
		exp = 23
	case size <= 32768<<20: // 16-32 GB: 16 MiB pieces (2^24)
		exp = 24
	case size <= 65536<<20: // 32-64 GB: 32 MiB pieces (2^25)
		exp = 25
	case size <= 131072<<20: // 64-128 GB: 64 MiB pieces (2^26)
		exp = 26
	default: // above 128 GB: 128 MiB pieces (2^27)
		exp = 27
	}

	// if no manual piece length was specified, cap at 2^24
	if maxPieceLength == nil && exp > 24 {
		exp = 24
	}

	// ensure we stay within bounds
	if exp > maxExp {
		exp = maxExp
	}

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

// CreateTorrent creates a new torrent file from the given options.
// Returns a Torrent struct containing the metainfo.
// This is the lower-level function; use Create() for a higher-level interface.
func CreateTorrent(opts CreateOptions) (*Torrent, error) {
	path := filepath.ToSlash(opts.Path)
	name := opts.Name
	if name == "" {
		// preserve the folder name even for single-file torrents
		name = filepath.Base(filepath.Clean(path))
	}

	mi := &metainfo.MetaInfo{
		Comment: opts.Comment,
	}

	// Set tracker information
	if len(opts.TrackerURLs) > 0 {
		mi.Announce = opts.TrackerURLs[0]
		// Create announce list with all trackers in a single tier
		mi.AnnounceList = [][]string{opts.TrackerURLs}
	}

	if !opts.NoCreator {
		mi.CreatedBy = fmt.Sprintf("mkbrr/%s (https://github.com/autobrr/mkbrr)", opts.Version)
	}

	if !opts.NoDate {
		mi.CreationDate = time.Now().Unix()
	}

	files := make([]fileEntry, 0, 1)
	var totalSize int64
	var baseDir string
	originalPaths := make(map[string]string) // map resolved path -> original path for metainfo

	err := filepath.Walk(path, func(currentPath string, walkInfo os.FileInfo, walkErr error) error {
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

		if resolvedInfo.IsDir() {
			if baseDir == "" && currentPath == path { // only set baseDir for the initial path if it's a dir
				baseDir = currentPath
			}
			return nil
		}

		// it's a file (or a link pointing to one)
		shouldIgnore, err := shouldIgnoreFile(currentPath, opts.ExcludePatterns, opts.IncludePatterns) // ignore based on original path
		if err != nil {
			return fmt.Errorf("error processing file patterns for %q: %w", currentPath, err)
		}
		if shouldIgnore {
			return nil
		}

		// add the file using the resolved path for hashing, but store the original path for metainfo
		files = append(files, fileEntry{
			path:   resolvedPath, // use the actual content path for hashing
			length: resolvedInfo.Size(),
			offset: totalSize,
		})
		originalPaths[resolvedPath] = currentPath
		totalSize += resolvedInfo.Size()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking path: %w", err)
	}

	// sort files to ensure consistent order
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
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

	// Function to create torrent with given piece length
	createWithPieceLength := func(pieceLength uint) (*Torrent, error) {
		pieceLenInt := int64(1) << pieceLength
		numPieces := (totalSize + pieceLenInt - 1) / pieceLenInt

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

		var pieceHashes [][]byte
		hasher := NewPieceHasher(files, pieceLenInt, int(numPieces), display, opts.FailOnSeasonPackWarning)
		// Pass the specified or default worker count from opts
		if err := hasher.hashPieces(opts.Workers); err != nil {
			return nil, err
		}
		pieceHashes = hasher.pieces

		info := &metainfo.Info{
			Name:        name,
			PieceLength: pieceLenInt,
			Private:     &opts.IsPrivate,
		}

		if opts.Source != "" {
			info.Source = opts.Source
		}

		info.Pieces = make([]byte, len(pieceHashes)*20)
		for i, piece := range pieceHashes {
			copy(info.Pieces[i*20:], piece)
		}

		if len(files) == 1 {
			// check if the input path is a directory
			pathInfo, err := os.Stat(path)
			if err != nil {
				return nil, fmt.Errorf("error checking path: %w", err)
			}

			if pathInfo.IsDir() {
				// if it's a directory, use the folder structure even for single files
				info.Files = make([]metainfo.FileInfo, 1)
				// Use the original path for calculating relative path in metainfo
				originalFilepath := originalPaths[files[0].path]
				if originalFilepath == "" {
					originalFilepath = files[0].path // Fallback if mapping missing
				}
				relPath, _ := filepath.Rel(baseDir, originalFilepath)
				pathComponents := strings.Split(filepath.ToSlash(relPath), "/") // Ensure forward slashes
				info.Files[0] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: files[0].length, // Length comes from resolved file
				}
			} else {
				// if it's a single file directly, use the simple format
				info.Length = files[0].length
			}
		} else {
			info.Files = make([]metainfo.FileInfo, len(files))
			for i, f := range files {
				// Use the original path for calculating relative path in metainfo
				originalFilepath := originalPaths[f.path]
				if originalFilepath == "" {
					originalFilepath = f.path // Fallback if mapping missing
				}
				relPath, _ := filepath.Rel(baseDir, originalFilepath)
				pathComponents := strings.Split(filepath.ToSlash(relPath), "/") // Ensure forward slashes
				info.Files[i] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: f.length, // Length comes from resolved file
				}
			}
		}

		infoBytes, err := bencode.Marshal(info)
		if err != nil {
			return nil, fmt.Errorf("error encoding info: %w", err)
		}

		// add random entropy field for cross-seeding if enabled
		if opts.Entropy {
			infoMap := make(map[string]interface{})
			if err := bencode.Unmarshal(infoBytes, &infoMap); err == nil {
				if entropy, err := generateRandomString(); err == nil {
					infoMap["entropy"] = entropy
					if infoBytes, err = bencode.Marshal(infoMap); err == nil {
						mi.InfoBytes = infoBytes
					}
				}
			}
		} else {
			mi.InfoBytes = infoBytes
		}

		if len(opts.WebSeeds) > 0 {
			mi.UrlList = opts.WebSeeds
		}

		return &Torrent{mi}, nil
	}

	var pieceLength uint
	if opts.PieceLengthExp == nil {
		if opts.MaxPieceLength != nil {
			// Get tracker's max piece length if available
			maxExp := uint(27) // absolute max 128 MiB
			if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
				if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(opts.TrackerURLs[0]); ok {
					maxExp = trackerMaxExp
				}
			}

			if *opts.MaxPieceLength < 14 || *opts.MaxPieceLength > maxExp {
				return nil, fmt.Errorf("max piece length exponent must be between 14 (16 KiB) and %d (%d MiB), got: %d",
					maxExp, 1<<(maxExp-20), *opts.MaxPieceLength)
			}
		}
		pieceLength = calculatePieceLength(totalSize, opts.MaxPieceLength, opts.TrackerURLs, opts.Verbose)
	} else {
		pieceLength = *opts.PieceLengthExp

		// Get tracker's max piece length if available
		maxExp := uint(27) // absolute max 128 MiB
		if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
			if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(opts.TrackerURLs[0]); ok {
				maxExp = trackerMaxExp
			}
		}

		if pieceLength < 16 || pieceLength > maxExp {
			if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
				return nil, fmt.Errorf("piece length exponent must be between 16 (64 KiB) and %d (%d MiB) for %s, got: %d",
					maxExp, 1<<(maxExp-20), opts.TrackerURLs[0], pieceLength)
			}
			return nil, fmt.Errorf("piece length exponent must be between 16 (64 KiB) and %d (%d MiB), got: %d",
				maxExp, 1<<(maxExp-20), pieceLength)
		}

		// If we have a tracker with specific ranges, show that we're using them and check if piece length matches
		if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
			if exp, ok := trackers.GetTrackerPieceSizeExp(opts.TrackerURLs[0], uint64(totalSize)); ok {
				if exp < 16 || exp > maxExp {
					return nil, fmt.Errorf("piece length exponent %d for %s is outside allowed range 16-%d", exp, opts.TrackerURLs[0], maxExp)
				}
				if opts.Verbose || opts.InfoOnly {
					display := NewDisplay(NewFormatter(opts.Verbose || opts.InfoOnly))
					display.SetQuiet(opts.Quiet || opts.InfoOnly)
					display.ShowMessage(fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
						totalSize>>20, formatPieceSize(exp)))
					fmt.Fprintln(display.output)
					if pieceLength != exp {
						display.ShowWarning(fmt.Sprintf("custom piece length %s differs from recommendation",
							formatPieceSize(pieceLength)))
					}
				}
			}
		}
	}

	// Check for tracker size limits and adjust piece length if needed
	if len(opts.TrackerURLs) > 0 && opts.TrackerURLs[0] != "" {
		if maxSize, ok := trackers.GetTrackerMaxTorrentSize(opts.TrackerURLs[0]); ok {
			// Try creating the torrent with initial piece length
			t, err := createWithPieceLength(pieceLength)
			if err != nil {
				return nil, err
			}

			// Check if it exceeds size limit
			torrentData, err := bencode.Marshal(t.MetaInfo)
			if err != nil {
				return nil, fmt.Errorf("error marshaling torrent data: %w", err)
			}

			// If it exceeds limit, try increasing piece length until it fits or we hit max
			for uint64(len(torrentData)) > maxSize && pieceLength < 24 {
				if opts.Verbose || opts.InfoOnly {
					display := NewDisplay(NewFormatter(opts.Verbose || opts.InfoOnly))
					display.SetQuiet(opts.Quiet || opts.InfoOnly)
					display.ShowWarning(fmt.Sprintf("increasing piece length to reduce torrent size (current: %.1f KiB, limit: %.1f KiB)",
						float64(len(torrentData))/(1<<10), float64(maxSize)/(1<<10)))
				}

				pieceLength++
				t, err = createWithPieceLength(pieceLength)
				if err != nil {
					return nil, err
				}

				torrentData, err = bencode.Marshal(t.MetaInfo)
				if err != nil {
					return nil, fmt.Errorf("error marshaling torrent data: %w", err)
				}
			}

			if uint64(len(torrentData)) > maxSize {
				return nil, fmt.Errorf("unable to create torrent under size limit (%.1f KiB) even with maximum piece length",
					float64(maxSize)/(1<<10))
			}

			return t, nil
		}
	}

	// No size limit, just create with original piece length
	return createWithPieceLength(pieceLength)
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

	// set name if not provided
	if opts.Name == "" {
		opts.Name = filepath.Base(filepath.Clean(opts.Path))
	}

	fileName := opts.Name
	if len(opts.TrackerURLs) == 1 && !opts.SkipPrefix {
		fileName = preset.GetDomainPrefix(opts.TrackerURLs[0]) + "_" + opts.Name
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
