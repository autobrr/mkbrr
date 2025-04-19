package torrent

// Displayer defines the interface for displaying progress during torrent creation
import (
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

// Displayer defines the interface for displaying progress during torrent creation
type Displayer interface {
	ShowProgress(total int)
	UpdateProgress(completed int, hashrate float64)
	ShowFiles(files []FileEntry, numWorkers int) // Use exported FileEntry type
	ShowSeasonPackWarnings(info *SeasonPackInfo)
	FinishProgress()
	IsBatch() bool
	// Add other methods used by CLI displayer if needed for completeness,
	// even if GUI implementation is empty.
	ShowMessage(msg string)
	ShowWarning(msg string)
	ShowOutputPathWithTime(path string, t time.Duration)
	ShowTorrentInfo(t *Torrent, info *metainfo.Info)
	ShowFileTree(info *metainfo.Info)
	ShowBatchResults(results []BatchResult, totalTime time.Duration)
}
