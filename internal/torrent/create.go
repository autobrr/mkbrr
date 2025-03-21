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
func calculatePieceLength(totalSize int64, maxPieceLength *uint, trackerURL string, verbose bool) uint {
	minExp := uint(16)
	maxExp := uint(24) // default max 16 MiB for automatic calculation, can be overridden up to 2^27

	// check if tracker has a maximum piece length constraint
	if trackerURL != "" {
		if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(trackerURL); ok {
			maxExp = trackerMaxExp
		}

		// check if tracker has specific piece size ranges
		if exp, ok := trackers.GetTrackerPieceSizeExp(trackerURL, uint64(totalSize)); ok {
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

func CreateTorrent(opts CreateTorrentOptions) (*Torrent, error) {
	path := filepath.ToSlash(opts.Path)
	name := opts.Name
	if name == "" {
		// preserve the folder name even for single-file torrents
		name = filepath.Base(filepath.Clean(path))
	}

	mi := &metainfo.MetaInfo{
		Announce: opts.TrackerURL,
		Comment:  opts.Comment,
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

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if baseDir == "" {
				baseDir = filePath
			}
			return nil
		}
		if shouldIgnoreFile(filePath) {
			return nil
		}
		files = append(files, fileEntry{
			path:   filePath,
			length: info.Size(),
			offset: totalSize,
		})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking path: %w", err)
	}

	// Sort files to ensure consistent order
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	// Function to create torrent with given piece length
	createWithPieceLength := func(pieceLength uint) (*Torrent, error) {
		pieceLenInt := int64(1) << pieceLength
		numPieces := (totalSize + pieceLenInt - 1) / pieceLenInt

		display := NewDisplay(NewFormatter(opts.Verbose))
		display.SetQuiet(opts.Quiet)
		hasher := NewPieceHasher(files, pieceLenInt, int(numPieces), display)

		if err := hasher.hashPieces(1); err != nil {
			return nil, fmt.Errorf("error hashing pieces: %w", err)
		}

		info := &metainfo.Info{
			Name:        name,
			PieceLength: pieceLenInt,
			Private:     &opts.IsPrivate,
		}

		if opts.Source != "" {
			info.Source = opts.Source
		}

		info.Pieces = make([]byte, len(hasher.pieces)*20)
		for i, piece := range hasher.pieces {
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
				relPath, _ := filepath.Rel(baseDir, files[0].path)
				pathComponents := strings.Split(relPath, string(filepath.Separator))
				info.Files[0] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: files[0].length,
				}
			} else {
				// if it's a single file directly, use the simple format
				info.Length = files[0].length
			}
		} else {
			info.Files = make([]metainfo.FileInfo, len(files))
			for i, f := range files {
				relPath, _ := filepath.Rel(baseDir, f.path)
				pathComponents := strings.Split(relPath, string(filepath.Separator))
				info.Files[i] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: f.length,
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
			if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(opts.TrackerURL); ok {
				maxExp = trackerMaxExp
			}

			if *opts.MaxPieceLength < 14 || *opts.MaxPieceLength > maxExp {
				return nil, fmt.Errorf("max piece length exponent must be between 14 (16 KiB) and %d (%d MiB), got: %d",
					maxExp, 1<<(maxExp-20), *opts.MaxPieceLength)
			}
		}
		pieceLength = calculatePieceLength(totalSize, opts.MaxPieceLength, opts.TrackerURL, opts.Verbose)
	} else {
		pieceLength = *opts.PieceLengthExp

		// Get tracker's max piece length if available
		maxExp := uint(27) // absolute max 128 MiB
		if trackerMaxExp, ok := trackers.GetTrackerMaxPieceLength(opts.TrackerURL); ok {
			maxExp = trackerMaxExp
		}

		if pieceLength < 16 || pieceLength > maxExp {
			if opts.TrackerURL != "" {
				return nil, fmt.Errorf("piece length exponent must be between 16 (64 KiB) and %d (%d MiB) for %s, got: %d",
					maxExp, 1<<(maxExp-20), opts.TrackerURL, pieceLength)
			}
			return nil, fmt.Errorf("piece length exponent must be between 16 (64 KiB) and %d (%d MiB), got: %d",
				maxExp, 1<<(maxExp-20), pieceLength)
		}

		// If we have a tracker with specific ranges, show that we're using them and check if piece length matches
		if exp, ok := trackers.GetTrackerPieceSizeExp(opts.TrackerURL, uint64(totalSize)); ok {
			if opts.Verbose {
				display := NewDisplay(NewFormatter(opts.Verbose))
				display.SetQuiet(opts.Quiet)
				display.ShowMessage(fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
					totalSize>>20, formatPieceSize(exp)))
				if pieceLength != exp {
					display.ShowWarning(fmt.Sprintf("custom piece length %s differs from recommendation",
						formatPieceSize(pieceLength)))
				}
			}
		}
	}

	// Check for tracker size limits and adjust piece length if needed
	if maxSize, ok := trackers.GetTrackerMaxTorrentSize(opts.TrackerURL); ok {
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
			if opts.Verbose {
				display := NewDisplay(NewFormatter(opts.Verbose))
				display.SetQuiet(opts.Quiet)
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

	// No size limit, just create with original piece length
	return createWithPieceLength(pieceLength)
}

// Create creates a new torrent file with the given options
func Create(opts CreateTorrentOptions) (*TorrentInfo, error) {
	// validate input path
	if _, err := os.Stat(opts.Path); err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", opts.Path, err)
	}

	// set name if not provided
	if opts.Name == "" {
		opts.Name = filepath.Base(filepath.Clean(opts.Path))
	}

	if opts.OutputPath == "" {
		fileName := opts.Name
		if opts.TrackerURL != "" && !opts.SkipPrefix {
			fileName = preset.GetDomainPrefix(opts.TrackerURL) + "_" + opts.Name
		}
		opts.OutputPath = fileName + ".torrent"
	} else if !strings.HasSuffix(opts.OutputPath, ".torrent") {
		opts.OutputPath = opts.OutputPath + ".torrent"
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
		Announce: opts.TrackerURL,
	}

	// display info if verbose
	if opts.Verbose {
		display := NewDisplay(NewFormatter(opts.Verbose))
		display.ShowTorrentInfo(t, info)
		if len(info.Files) > 0 {
			display.ShowFileTree(info)
		}
	}

	return torrentInfo, nil
}
