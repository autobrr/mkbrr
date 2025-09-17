package torrent

// Displayer defines the interface for displaying progress during torrent creation
type Displayer interface {
	ShowProgress(total int)
	UpdateProgress(completed int, hashrate float64)
	ShowFiles(files []fileEntry, numWorkers int)
	ShowSeasonPackWarnings(info *SeasonPackInfo)
	FinishProgress()
	IsBatch() bool
}
