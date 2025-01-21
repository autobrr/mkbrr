package torrent

// Displayer defines the interface for displaying progress during torrent creation
type Displayer interface {
	ShowProgress(total int)
	UpdateProgress(completed int)
	FinishProgress()
	IsBatch() bool
}
