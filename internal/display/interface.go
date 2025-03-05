package display

import "github.com/autobrr/mkbrr/internal/types"

// Displayer defines an interface for display functionality
type Displayer interface {
	ShowProgress(total int)
	UpdateProgress(completed int, hashrate float64)
	ShowFiles(files []types.EntryFile)
	FinishProgress()
	IsBatch() bool
	SetBatch(isBatch bool)
	ShowMessage(msg string)
	ShowError(msg string)
	ShowWarning(msg string)
	ShowSeasonPackWarnings(info *SeasonPackInfo)
}

// SeasonPackInfo represents information about a season pack
type SeasonPackInfo struct {
	IsSeasonPack    bool
	Season          int
	Episodes        []int
	MissingEpisodes []int
	MaxEpisode      int
	VideoFileCount  int
	IsSuspicious    bool
}

// TorrentDisplayer defines an interface for displaying torrent information
type TorrentDisplayer interface {
	ShowTorrentInfo(t *types.Torrent, info interface{})
	ShowFileTree(info interface{})
	ShowOutputPathWithTime(path string, duration interface{})
	ShowBatchResults(results interface{}, duration interface{})
}
