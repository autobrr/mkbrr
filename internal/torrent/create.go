package torrent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/display"
	"github.com/autobrr/mkbrr/internal/torrentutils"
	"github.com/autobrr/mkbrr/internal/trackers"
	"github.com/autobrr/mkbrr/internal/types"
	"github.com/autobrr/mkbrr/internal/utils"
)

// Variables for display functionality - filled by Init
var displayCallback func(verbose bool) display.Displayer

// Init sets up display callback functions
func Init(displayFunc func(verbose bool) display.Displayer) {
	displayCallback = displayFunc
}

// GetTorrentInfo extracts and returns the metainfo.Info from a Torrent
func GetTorrentInfo(t *types.Torrent) *metainfo.Info {
	info := &metainfo.Info{}
	_ = bencode.Unmarshal(t.InfoBytes, info)
	return info
}

func CreateTorrent(opts types.CreateTorrentOptions) (*types.Torrent, error) {
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
		mi.CreatedBy = fmt.Sprintf("mkbrr/%s", opts.Version)
	}

	if !opts.NoDate {
		mi.CreationDate = time.Now().Unix()
	}

	files := make([]types.EntryFile, 0, 1)
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
		files = append(files, types.EntryFile{
			Path:   filePath,
			Length: info.Size(),
			Offset: totalSize,
		})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking path: %w", err)
	}

	// Sort files to ensure consistent order
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	// Function to create torrent with given piece length
	createWithPieceLength := func(pieceLength uint) (*types.Torrent, error) {
		pieceLenInt := int64(1) << pieceLength
		numPieces := (totalSize + pieceLenInt - 1) / pieceLenInt

		var display display.Displayer
		if displayCallback != nil {
			display = displayCallback(opts.Verbose)
		} else {
			// Create a no-op displayer if display callback is not set
			display = &noopDisplayer{}
		}

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
				relPath, _ := filepath.Rel(baseDir, files[0].Path)
				pathComponents := strings.Split(relPath, string(filepath.Separator))
				info.Files[0] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: files[0].Length,
				}
			} else {
				// if it's a single file directly, use the simple format
				info.Length = files[0].Length
			}
		} else {
			info.Files = make([]metainfo.FileInfo, len(files))
			for i, f := range files {
				relPath, _ := filepath.Rel(baseDir, f.Path)
				pathComponents := strings.Split(relPath, string(filepath.Separator))
				info.Files[i] = metainfo.FileInfo{
					Path:   pathComponents,
					Length: f.Length,
				}
			}
		}

		infoBytes, err := bencode.Marshal(info)
		if err != nil {
			return nil, fmt.Errorf("error encoding info: %w", err)
		}
		mi.InfoBytes = infoBytes

		if len(opts.WebSeeds) > 0 {
			mi.UrlList = opts.WebSeeds
		}

		return &types.Torrent{MetaInfo: mi}, nil
	}

	var pieceLength uint
	if opts.PieceLengthExp == nil {
		// Use validation utility to calculate optimal piece length
		pieceLength = torrentutils.CalculatePieceLength(totalSize, opts.MaxPieceLength, opts.TrackerURL)

		// Display message about tracker-specific ranges if applicable
		if opts.Verbose && displayCallback != nil {
			display := displayCallback(opts.Verbose)
			if exp, ok := trackers.GetTrackerPieceSizeExp(opts.TrackerURL, uint64(totalSize)); ok {
				display.ShowMessage(fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
					totalSize>>20, utils.FormatPieceSize(exp)))
			}
		}
	} else {
		pieceLength = *opts.PieceLengthExp

		// Validate the piece length
		if err := torrentutils.ValidatePieceLength(pieceLength, opts.TrackerURL); err != nil {
			return nil, err
		}

		// Show warning if piece length doesn't match tracker recommendation
		if opts.Verbose && displayCallback != nil {
			display := displayCallback(opts.Verbose)
			if exp, ok := trackers.GetTrackerPieceSizeExp(opts.TrackerURL, uint64(totalSize)); ok {
				display.ShowMessage(fmt.Sprintf("using tracker-specific range for content size: %d MiB (recommended: %s pieces)",
					totalSize>>20, utils.FormatPieceSize(exp)))
				if pieceLength != exp {
					display.ShowWarning(fmt.Sprintf("custom piece length %s differs from recommendation",
						utils.FormatPieceSize(pieceLength)))
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
			if opts.Verbose && displayCallback != nil {
				display := displayCallback(opts.Verbose)
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
func Create(opts types.CreateTorrentOptions) (*types.TorrentInfo, error) {
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
		if opts.TrackerURL != "" {
			fileName = utils.GetDomainPrefix(opts.TrackerURL) + "_" + opts.Name
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
	info := GetTorrentInfo(t)

	// create torrent info for return
	torrentInfo := &types.TorrentInfo{
		Path:     opts.OutputPath,
		Size:     info.TotalLength(),
		InfoHash: t.MetaInfo.HashInfoBytes().String(),
		Files:    len(info.Files),
		Announce: opts.TrackerURL,
		MetaInfo: t.MetaInfo,
	}

	// display info if verbose
	if opts.Verbose && displayCallback != nil {
		disp := displayCallback(opts.Verbose)
		if td, ok := disp.(display.TorrentDisplayer); ok {
			td.ShowTorrentInfo(t, info)
			if len(info.Files) > 0 {
				td.ShowFileTree(info)
			}
		}
	}

	return torrentInfo, nil
}

// noopDisplayer is a no-op implementation of interfaces.Displayer for when no display is provided
type noopDisplayer struct{}

func (n *noopDisplayer) ShowProgress(total int)                              {}
func (n *noopDisplayer) UpdateProgress(completed int, hashrate float64)      {}
func (n *noopDisplayer) ShowFiles(files []types.EntryFile)                   {}
func (n *noopDisplayer) FinishProgress()                                     {}
func (n *noopDisplayer) IsBatch() bool                                       { return false }
func (n *noopDisplayer) SetBatch(isBatch bool)                               {}
func (n *noopDisplayer) ShowMessage(msg string)                              {}
func (n *noopDisplayer) ShowError(msg string)                                {}
func (n *noopDisplayer) ShowWarning(msg string)                              {}
func (n *noopDisplayer) ShowSeasonPackWarnings(info *display.SeasonPackInfo) {}
