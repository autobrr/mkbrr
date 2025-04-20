package guiapp

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	mi "github.com/anacrolix/torrent/metainfo"

	"github.com/autobrr/mkbrr/internal/torrent"
)

type fyneDisplayer struct {
	progressDialog dialog.Dialog
	window         fyne.Window
	progressBar    *widget.ProgressBar
	totalPieces    int
}

func newFyneDisplayer(w fyne.Window, title, message string) *fyneDisplayer {
	progressBar := widget.NewProgressBar()
	progressDialog := dialog.NewCustom(title, "Cancel", container.NewVBox(widget.NewLabel(message), progressBar), w)
	return &fyneDisplayer{
		progressDialog: progressDialog,
		progressBar:    progressBar,
		window:         w,
	}
}
func (d *fyneDisplayer) ShowProgress(total int) {
	d.totalPieces = total
	if total == 0 {
		d.progressBar.Max = 1
		d.progressBar.SetValue(1)
	} else {
		d.progressBar.Max = float64(total)
		d.progressBar.SetValue(0)
	}
	d.progressDialog.Show()
}
func (d *fyneDisplayer) UpdateProgress(completed int, hashrate float64) {
	if d.totalPieces > 0 {
		d.progressBar.SetValue(float64(completed))
	}
}
func (d *fyneDisplayer) FinishProgress() {
	if d.progressDialog != nil {
		if d.totalPieces > 0 {
			d.progressBar.SetValue(float64(d.totalPieces))
		} else {
			d.progressBar.SetValue(1)
		}
		d.progressDialog.Hide()
	}
}
func (d *fyneDisplayer) ShowFiles(files []torrent.FileEntry, numWorkers int)                     {}
func (d *fyneDisplayer) ShowSeasonPackWarnings(info *torrent.SeasonPackInfo)                     {}
func (d *fyneDisplayer) IsBatch() bool                                                           { return false }
func (d *fyneDisplayer) ShowMessage(msg string)                                                  {}
func (d *fyneDisplayer) ShowWarning(msg string)                                                  {}
func (d *fyneDisplayer) ShowOutputPathWithTime(path string, t time.Duration)                     {}
func (d *fyneDisplayer) ShowTorrentInfo(t *torrent.Torrent, info *mi.Info)                       {}
func (d *fyneDisplayer) ShowFileTree(info *mi.Info)                                              {}
func (d *fyneDisplayer) ShowBatchResults(results []torrent.BatchResult, totalTime time.Duration) {}
