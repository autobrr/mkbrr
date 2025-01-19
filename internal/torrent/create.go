package torrent

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// min returns the smaller of x or y
func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

// max returns the larger of x or y
func max(x, y int64) int64 {
	if x > y {
		return x
	}
	return y
}

// calculatePieceLength calculates the optimal piece length based on total size
func calculatePieceLength(totalSize int64) uint {
	// ensure minimum of 1 byte and calculate exponent
	size := max(totalSize, 1)
	exp := uint(math.Ceil(math.Log2(float64(size)))/2.0 + 4.0)

	// ensure bounds: 16 KiB (2^14) to 16 MiB (2^24)
	exp = uint(min(max(int64(exp), 14), 24))

	//if verbose {
	//	pieceSize := int64(1) << exp
	//	pieceCount := (totalSize + pieceSize - 1) / pieceSize
	//	pieceListSize := pieceCount * 20 // SHA-1 hashes are 20 bytes each
	//
	//	fmt.Printf("Content size: %s (%d bytes)\n", humanize.Bytes(uint64(totalSize)), totalSize)
	//	fmt.Printf("Piece length: %s (%d bytes)\n", humanize.Bytes(uint64(pieceSize)), pieceSize)
	//	fmt.Printf("Piece count: %s\n", humanize.Comma(int64(pieceCount)))
	//	fmt.Printf("Piece list size: %s (%d bytes)\n", humanize.Bytes(uint64(pieceListSize)), pieceListSize)
	//}

	return exp
}

// GetInfo returns the info dictionary from the metainfo struct
func (t *Torrent) GetInfo() *metainfo.Info {
	info := &metainfo.Info{}
	_ = bencode.Unmarshal(t.InfoBytes, info)
	return info
}

// CreateTorrent creates a new torrent file with the given options
func CreateTorrent(opts CreateTorrentOptions) (*Torrent, error) {
	path := filepath.ToSlash(opts.Path)
	name := opts.Name
	if name == "" {
		name = filepath.Base(filepath.Clean(path))
	}

	mi := &metainfo.MetaInfo{
		CreatedBy: fmt.Sprintf("mkbrr/%s", opts.Version),
		Announce:  opts.TrackerURL,
		Comment:   opts.Comment,
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

	var pieceLength uint
	if opts.PieceLengthExp == nil {
		pieceLength = calculatePieceLength(totalSize)
	} else {
		// log the received value for debugging
		if opts.Verbose {
			fmt.Printf("Using requested piece length: 2^%d bytes\n", *opts.PieceLengthExp)
		}

		// enforce the piece length strictly
		pieceLength = *opts.PieceLengthExp

		// validate bounds - now allowing up to 2^24 (16 MiB)
		if pieceLength < 14 || pieceLength > 24 {
			return nil, fmt.Errorf("piece length exponent must be between 14 (16 KiB) and 24 (16 MiB), got: %d", pieceLength)
		}
	}

	pieceLenInt := int64(1) << pieceLength
	numPieces := (totalSize + pieceLenInt - 1) / pieceLenInt

	hasher := &pieceHasher{
		pieces:    make([][]byte, numPieces),
		pieceLen:  pieceLenInt,
		numPieces: int(numPieces),
		files:     files,
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 4 {
		numWorkers = 4
	}
	if err := hasher.hashPieces(numWorkers); err != nil {
		return nil, fmt.Errorf("error hashing pieces: %w", err)
	}

	info := &metainfo.Info{
		Name:        name,
		PieceLength: pieceLenInt,
		Private:     &opts.IsPrivate,
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	if opts.Source != "" {
		info.Source = opts.Source
	}

	info.Pieces = make([]byte, len(hasher.pieces)*20)
	for i, piece := range hasher.pieces {
		copy(info.Pieces[i*20:], piece)
	}

	if len(files) == 1 {
		info.Length = files[0].length
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
	mi.InfoBytes = infoBytes

	if len(opts.WebSeeds) > 0 {
		mi.UrlList = opts.WebSeeds
	}

	return &Torrent{mi}, nil
}
