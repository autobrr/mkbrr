package torrent

import (
	"os"

	"github.com/anacrolix/torrent/metainfo"
)

// CreateTorrentOptions contains all options for creating a torrent
type CreateTorrentOptions struct {
	PieceLengthExp  *uint
	MaxPieceLength  *uint
	Path            string
	Name            string
	TrackerURL      string
	Comment         string
	Source          string
	Version         string
	OutputPath      string
	WebSeeds        []string
	ExcludePatterns []string
	IncludePatterns []string
	IsPrivate       bool
	NoDate          bool
	NoCreator       bool
	Verbose         bool
	Entropy         bool
	Quiet           bool
	SkipPrefix      bool
	Workers         int
}

// Torrent represents a torrent file with additional functionality
type Torrent struct {
	*metainfo.MetaInfo
}

// FileEntry represents a file in the torrent
type FileEntry struct {
	Name string
	Path string
	Size int64
}

// internal file entry for processing
type fileEntry struct {
	path   string
	length int64
	offset int64
}

// internal file reader for processing
type fileReader struct {
	file     *os.File
	position int64
	length   int64
}

// TorrentInfo contains summary information about the created torrent
type TorrentInfo struct {
	MetaInfo *metainfo.MetaInfo
	Path     string
	InfoHash string
	Announce string
	Size     int64
	Files    int
}

// VerificationResult holds the outcome of a torrent data verification check
type VerificationResult struct {
	BadPieceIndices []int
	MissingFiles    []string
	TotalPieces     int
	GoodPieces      int
	BadPieces       int
	MissingPieces   int
	Completion      float64
}
