package torrent

import (
	"os"

	"github.com/anacrolix/torrent/metainfo"
)

// CreateTorrentOptions contains all options for creating a torrent
type CreateTorrentOptions struct {
	Path            string
	Name            string
	TrackerURL      string
	WebSeeds        []string
	IsPrivate       bool
	Comment         string
	PieceLengthExp  *uint
	MaxPieceLength  *uint
	Source          string
	NoDate          bool
	NoCreator       bool
	Verbose         bool
	Version         string
	OutputPath      string
	Entropy         bool
	Quiet           bool
	SkipPrefix      bool
	ExcludePatterns []string
	IncludePatterns []string
}

// Torrent represents a torrent file with additional functionality
type Torrent struct {
	*metainfo.MetaInfo
}

// FileEntry represents a file in the torrent
type FileEntry struct {
	Name string
	Size int64
	Path string
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
	Path     string
	Size     int64
	InfoHash string
	Files    int
	Announce string
	MetaInfo *metainfo.MetaInfo
}
