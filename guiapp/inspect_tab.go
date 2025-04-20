package guiapp

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/autobrr/mkbrr/internal/torrent"
)

func inspectTorrentTab(w fyne.Window) fyne.CanvasObject {
	selectedFileLabel := widget.NewLabel("No torrent file selected")
	selectedFileLabel.Wrapping = fyne.TextWrapBreak
	selectButton := widget.NewButtonWithIcon("Select Torrent File", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			selectedFileLabel.SetText(uri.URI().Path())
		}, w)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"}))
		fileDialog.Resize(fyne.NewSize(700, 500))
		fileDialog.Show()
	})
	infoText := widget.NewMultiLineEntry()
	infoText.Wrapping = fyne.TextWrapBreak
	infoText.SetPlaceHolder("Torrent information will be displayed here")
	infoText.Disable()
	formatter := torrent.NewBytesFormatter(false)
	inspectButton := widget.NewButtonWithIcon("Inspect Torrent", theme.SearchIcon(), func() {
		path := selectedFileLabel.Text
		if path == "No torrent file selected" {
			dialog.ShowError(fmt.Errorf("please select a torrent file"), w)
			return
		}
		progressBar := widget.NewProgressBar()
		progress := dialog.NewCustomWithoutButtons("Inspecting Torrent", container.NewVBox(widget.NewLabel("Loading torrent data..."), progressBar), w)
		progress.Show()
		go func() {
			defer progress.Hide()
			t, err := torrent.LoadFromFile(path)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			info := t.GetInfo()
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Name: %s\n", info.Name))
			sb.WriteString(fmt.Sprintf("Info Hash: %s\n", t.HashInfoBytes()))
			if t.CreationDate != 0 {
				sb.WriteString(fmt.Sprintf("Created: %s\n", time.Unix(t.CreationDate, 0).Format("2006-01-02 15:04:05 MST")))
			}
			sb.WriteString(fmt.Sprintf("Piece Length: %s\n", formatter.FormatBytes(info.PieceLength)))
			sb.WriteString(fmt.Sprintf("Pieces: %d\n", len(info.Pieces)/20))
			if info.Private != nil {
				sb.WriteString(fmt.Sprintf("Private: %t\n", *info.Private))
			}
			sb.WriteString(fmt.Sprintf("Total Size: %s\n", formatter.FormatBytes(info.TotalLength())))
			if t.Comment != "" {
				sb.WriteString(fmt.Sprintf("\nComment: %s\n", t.Comment))
			}
			if info.Source != "" {
				sb.WriteString(fmt.Sprintf("Source: %s\n", info.Source))
			}
			if t.CreatedBy != "" {
				sb.WriteString(fmt.Sprintf("Created By: %s\n", t.CreatedBy))
			}
			if len(t.AnnounceList) > 0 {
				sb.WriteString("\nTrackers:\n")
				for _, tier := range t.AnnounceList {
					for _, tracker := range tier {
						sb.WriteString(fmt.Sprintf("- %s\n", tracker))
					}
				}
			} else if t.Announce != "" {
				sb.WriteString(fmt.Sprintf("\nTracker: %s\n", t.Announce))
			}
			if len(info.Files) > 0 {
				sb.WriteString(fmt.Sprintf("\nFiles: %d\n", len(info.Files)))
				for _, file := range info.Files {
					filePath := filepath.Join(file.Path...)
					sb.WriteString(fmt.Sprintf("- %s (%s)\n", filePath, formatter.FormatBytes(file.Length)))
				}
			} else if info.Length > 0 {
				sb.WriteString(fmt.Sprintf("\nFile: %s (%s)\n", info.Name, formatter.FormatBytes(info.Length)))
			}
			infoText.SetText(sb.String())
			infoText.Enable()
		}()
	})
	topContainer := container.NewVBox(widget.NewLabelWithStyle("Inspect a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), container.NewVBox(selectedFileLabel, selectButton), widget.NewSeparator(), inspectButton)
	scrollContainer := container.NewScroll(infoText)
	split := container.NewVSplit(topContainer, scrollContainer)
	split.Offset = 0.3
	return container.NewPadded(split)
}
