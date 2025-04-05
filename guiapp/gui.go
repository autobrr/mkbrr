package guiapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/anacrolix/torrent/bencode"
	mi "github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
)

func Run(version, buildTime, appName string) {
	a := app.NewWithID("com.autobrr.mkbrr")
	w := a.NewWindow(appName)
	w.Resize(fyne.NewSize(800, 600))
	w.SetMaster()

	createTab := createTorrentTab(w, version, appName)
	inspectTab := inspectTorrentTab(w)
	modifyTab := modifyTorrentTab(w)

	tabs := container.NewAppTabs(
		container.NewTabItem("Create", createTab),
		container.NewTabItem("Inspect", inspectTab),
		container.NewTabItem("Modify", modifyTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.ShowAndRun() // This blocks until the app closes
}

type fyneDisplayer struct {
	progressDialog dialog.Dialog
	progressBar    *widget.ProgressBar
	totalPieces    int
}

func newFyneDisplayer(w fyne.Window, title, message string) *fyneDisplayer {
	progressBar := widget.NewProgressBar()
	progressDialog := dialog.NewCustom(title, "Cancel", container.NewVBox(widget.NewLabel(message), progressBar), w)
	return &fyneDisplayer{
		progressDialog: progressDialog,
		progressBar:    progressBar,
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
func (d *fyneDisplayer) ShowFiles(files []torrent.FileEntry)                                     {}
func (d *fyneDisplayer) ShowSeasonPackWarnings(info *torrent.SeasonPackInfo)                     {}
func (d *fyneDisplayer) IsBatch() bool                                                           { return false }
func (d *fyneDisplayer) ShowMessage(msg string)                                                  {}
func (d *fyneDisplayer) ShowWarning(msg string)                                                  {}
func (d *fyneDisplayer) ShowOutputPathWithTime(path string, t time.Duration)                     {}
func (d *fyneDisplayer) ShowTorrentInfo(t *torrent.Torrent, info *mi.Info)                       {}
func (d *fyneDisplayer) ShowFileTree(info *mi.Info)                                              {}
func (d *fyneDisplayer) ShowBatchResults(results []torrent.BatchResult, totalTime time.Duration) {}

func createTorrentTab(w fyne.Window, version, appName string) fyne.CanvasObject {
	var presetConfig *preset.Config
	presetPath, err := preset.FindPresetFile("")
	if err == nil {
		presetConfig, err = preset.Load(presetPath)
		if err != nil {
			log.Printf("Error loading presets from %s: %v\n", presetPath, err)
		} else {
			log.Printf("Loaded presets from %s\n", presetPath)
		}
	} else {
		log.Println("No preset file found.")
	}

	selectedPathLabel := widget.NewLabel("No path selected")
	selectedPathLabel.Wrapping = fyne.TextWrapBreak
	selectDirButton := widget.NewButtonWithIcon("Select Directory", theme.FolderOpenIcon(), func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			selectedPathLabel.SetText(uri.Path())
		}, w)
		folderDialog.Resize(fyne.NewSize(700, 500))
		folderDialog.Show()
	})
	selectFileButton := widget.NewButtonWithIcon("Select File", theme.FileIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			selectedPathLabel.SetText(uri.URI().Path())
		}, w)
		fileDialog.Resize(fyne.NewSize(700, 500))
		fileDialog.Show()
	})
	selectButtons := container.NewHBox(selectDirButton, selectFileButton)
	trackerEntry := widget.NewEntry()
	trackerEntry.SetPlaceHolder("https://example.com/announce")
	pieceSizeOptions := []string{"Auto", "16 KiB", "32 KiB", "64 KiB", "128 KiB", "256 KiB", "512 KiB", "1 MiB", "2 MiB", "4 MiB", "8 MiB", "16 MiB", "32 MiB", "64 MiB", "128 MiB"}
	pieceSizeSelect := widget.NewSelect(pieceSizeOptions, func(value string) {})
	pieceSizeSelect.SetSelectedIndex(0)
	privateCheck := widget.NewCheck("Private", nil)
	privateCheck.SetChecked(true)
	sourceEntry := widget.NewEntry()
	commentEntry := widget.NewEntry()
	commentEntry.MultiLine = true
	commentEntry.Wrapping = fyne.TextWrapBreak
	excludeEntry := widget.NewEntry()
	excludeEntry.SetPlaceHolder("*.nfo, *.jpg, temp* (comma-separated)")
	includeEntry := widget.NewEntry()
	includeEntry.SetPlaceHolder("*.mkv, *.mp4 (comma-separated)")
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("auto-generated filename")
	outputBrowseButton := widget.NewButton("Browse...", func() {
		saveDialog := dialog.NewFileSave(func(uri fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			outputEntry.SetText(uri.URI().Path())
		}, w)
		currentOutput := outputEntry.Text
		if currentOutput == "" {
			inputPath := selectedPathLabel.Text
			if inputPath != "No path selected" {
				currentOutput = filepath.Base(filepath.Clean(inputPath)) + ".torrent"
			} else {
				currentOutput = "new.torrent"
			}
		}
		saveDialog.SetFileName(currentOutput)
		if dir := filepath.Dir(currentOutput); dir != "." && dir != "/" {
			listableURI, err := storage.ListerForURI(storage.NewFileURI(dir))
			if err == nil {
				saveDialog.SetLocation(listableURI)
			}
		}
		saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"}))
		saveDialog.Resize(fyne.NewSize(700, 500))
		saveDialog.Show()
	})
	outputContainer := container.NewBorder(nil, nil, nil, outputBrowseButton, outputEntry)
	randomizeCheck := widget.NewCheck("Randomize Info Hash", nil)

	presetSelect := widget.NewSelect([]string{"Manual"}, func(selectedName string) {
		if presetConfig == nil || selectedName == "Manual" {
			return
		}

		opts, err := presetConfig.GetPreset(selectedName)
		if err != nil {
			log.Printf("Error getting preset %s: %v\n", selectedName, err)
			dialog.ShowError(fmt.Errorf("failed to load preset '%s': %w", selectedName, err), w)
			return
		}

		if len(opts.Trackers) > 0 {
			trackerEntry.SetText(opts.Trackers[0])
		} else {
			trackerEntry.SetText("")
		}
		if opts.Private != nil {
			privateCheck.SetChecked(*opts.Private)
		}
		sourceEntry.SetText(opts.Source)
		commentEntry.SetText(opts.Comment)
		excludeEntry.SetText(strings.Join(opts.ExcludePatterns, ", "))
		includeEntry.SetText(strings.Join(opts.IncludePatterns, ", "))
	})
	presetSelect.PlaceHolder = "Select Preset (Optional)"

	if presetConfig != nil {
		presetNames := []string{"Manual"}
		keys := make([]string, 0, len(presetConfig.Presets))
		for k := range presetConfig.Presets {
			keys = append(keys, k)
		}
		sort.Strings(keys) // Sort preset names alphabetically
		presetNames = append(presetNames, keys...)
		presetSelect.Options = presetNames
		presetSelect.SetSelectedIndex(0) // Default to "Manual"
	} else {
		presetSelect.Disable()
	}

	createButton := widget.NewButtonWithIcon("Create Torrent", theme.DocumentCreateIcon(), func() {
		path := selectedPathLabel.Text
		if path == "No path selected" {
			dialog.ShowError(fmt.Errorf("please select a file or directory"), w)
			return
		}
		trackerURL := trackerEntry.Text
		if trackerURL == "" {
			dialog.ShowError(fmt.Errorf("tracker URL is required"), w)
			return
		}
		outputPath := outputEntry.Text
		if outputPath == "" {
			baseName := filepath.Base(path)
			outputPath = baseName + ".torrent"
			outputEntry.SetText(outputPath)
		}

		var pieceSize uint
		switch pieceSizeSelect.Selected {
		case "Auto":
			pieceSize = 0
		case "16 KiB":
			pieceSize = 14
		case "32 KiB":
			pieceSize = 15
		case "64 KiB":
			pieceSize = 16
		case "128 KiB":
			pieceSize = 17
		case "256 KiB":
			pieceSize = 18
		case "512 KiB":
			pieceSize = 19
		case "1 MiB":
			pieceSize = 20
		case "2 MiB":
			pieceSize = 21
		case "4 MiB":
			pieceSize = 22
		case "8 MiB":
			pieceSize = 23
		case "16 MiB":
			pieceSize = 24
		case "32 MiB":
			pieceSize = 25
		case "64 MiB":
			pieceSize = 26
		case "128 MiB":
			pieceSize = 27
		default:
			pieceSize = 0
		}

		displayer := newFyneDisplayer(w, "Creating Torrent", "Hashing pieces...")
		isPrivate := privateCheck.Checked
		options := torrent.CreateTorrentOptions{
			Path:            path,
			OutputPath:      outputPath,
			IsPrivate:       isPrivate,
			Source:          sourceEntry.Text,
			Comment:         commentEntry.Text,
			TrackerURL:      trackerURL,
			Displayer:       displayer,
			Version:         version,
			AppName:         appName,
			Entropy:         randomizeCheck.Checked,
			ExcludePatterns: parseExcludePatterns(excludeEntry.Text),
			IncludePatterns: parseIncludePatterns(includeEntry.Text),
		}
		if pieceSize > 0 {
			options.PieceLengthExp = &pieceSize
		}

		go func() {
			result, err := torrent.Create(options)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Torrent Created", fmt.Sprintf("Successfully created torrent: %s\n\nInfo Hash: %s", outputPath, result.InfoHash), w)
			selectedPathLabel.SetText("No path selected")
			trackerEntry.SetText("")
			sourceEntry.SetText("")
			commentEntry.SetText("")
			excludeEntry.SetText("")
			includeEntry.SetText("")
			outputEntry.SetText("")
			if presetConfig != nil {
				presetSelect.SetSelectedIndex(0)
			}
		}()
	})

	formItems := []*widget.FormItem{
		{Text: "Path", Widget: container.NewVBox(selectedPathLabel, selectButtons)},
	}

	if presetConfig != nil {
		formItems = append(formItems, &widget.FormItem{Text: "Preset", Widget: presetSelect})
	}

	formItems = append(formItems,
		&widget.FormItem{Text: "Tracker URL", Widget: trackerEntry},
		&widget.FormItem{Text: "Piece Size", Widget: pieceSizeSelect},
		&widget.FormItem{Text: "Private", Widget: privateCheck},
		&widget.FormItem{Text: "Source", Widget: sourceEntry},
		&widget.FormItem{Text: "Comment", Widget: commentEntry},
		&widget.FormItem{Text: "Exclude Patterns", Widget: excludeEntry},
		&widget.FormItem{Text: "Include Patterns", Widget: includeEntry},
		&widget.FormItem{Text: "Output File", Widget: outputContainer},
		&widget.FormItem{Text: "Randomize Hash", Widget: randomizeCheck},
	)

	form := &widget.Form{
		Items:      formItems,
		SubmitText: "Create Torrent",
		OnSubmit:   func() { createButton.OnTapped() },
	}

	content := container.NewVBox(widget.NewLabelWithStyle("Create a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), form)
	return container.NewPadded(content)
}

func parseExcludePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseIncludePatterns(patterns string) []string {
	if patterns == "" {
		return nil
	}
	parts := strings.Split(patterns, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

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

func modifyTorrentTab(w fyne.Window) fyne.CanvasObject {
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
	trackerEntry := widget.NewEntry()
	trackerEntry.SetPlaceHolder("https://example.com/announce")
	privateCheck := widget.NewCheck("Private", nil)
	privateCheck.SetChecked(true)
	sourceEntry := widget.NewEntry()
	commentEntry := widget.NewEntry()
	commentEntry.MultiLine = true
	commentEntry.Wrapping = fyne.TextWrapBreak
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Same as input (will overwrite)")
	outputBrowseButton := widget.NewButton("Browse...", func() {
		saveDialog := dialog.NewFileSave(func(uri fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return
			}
			outputEntry.SetText(uri.URI().Path())
		}, w)
		currentOutput := outputEntry.Text
		if currentOutput == "" {
			inputPath := selectedFileLabel.Text
			if inputPath != "No torrent file selected" {
				currentOutput = inputPath
			} else {
				currentOutput = "output.torrent"
			}
		}
		saveDialog.SetFileName(filepath.Base(currentOutput))
		if dir := filepath.Dir(currentOutput); dir != "." && dir != "/" {
			listableURI, err := storage.ListerForURI(storage.NewFileURI(dir))
			if err == nil {
				saveDialog.SetLocation(listableURI)
			}
		}
		saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"}))
		saveDialog.Resize(fyne.NewSize(700, 500))
		saveDialog.Show()
	})
	outputContainer := container.NewBorder(nil, nil, nil, outputBrowseButton, outputEntry)
	randomizeCheck := widget.NewCheck("Randomize Info Hash", nil)
	modifyButton := widget.NewButtonWithIcon("Modify Torrent", theme.DocumentSaveIcon(), func() {
		path := selectedFileLabel.Text
		if path == "No torrent file selected" {
			dialog.ShowError(fmt.Errorf("please select a torrent file"), w)
			return
		}
		progress := dialog.NewCustomWithoutButtons("Modifying Torrent", container.NewVBox(widget.NewLabel("Processing..."), widget.NewProgressBar()), w)
		progress.Show()
		outputPath := outputEntry.Text
		if outputPath == "" {
			outputPath = path
		}
		go func() {
			defer progress.Hide()
			t, err := torrent.LoadFromFile(path)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			modified := false
			if trackerEntry.Text != "" {
				t.Announce = trackerEntry.Text
				t.AnnounceList = [][]string{{trackerEntry.Text}}
				modified = true
			}
			if sourceEntry.Text != "" {
				info := t.GetInfo()
				info.Source = sourceEntry.Text
				infoBytes, err := bencode.Marshal(info)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
					return
				}
				t.InfoBytes = infoBytes
				modified = true
			}
			if commentEntry.Text != "" {
				t.Comment = commentEntry.Text
				modified = true
			}
			isPrivate := privateCheck.Checked
			info := t.GetInfo()
			if (info.Private == nil) || (*info.Private != isPrivate) {
				info.Private = &isPrivate
				infoBytes, err := bencode.Marshal(info)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
					return
				}
				t.InfoBytes = infoBytes
				modified = true
			}
			if randomizeCheck.Checked {
				infoMap := make(map[string]interface{})
				if err := bencode.Unmarshal(t.InfoBytes, &infoMap); err != nil {
					dialog.ShowError(fmt.Errorf("failed to decode info: %w", err), w)
					return
				}
				entropy, err := torrent.GenerateRandomString()
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to generate entropy: %w", err), w)
					return
				}
				infoMap["entropy"] = entropy
				infoBytes, err := bencode.Marshal(infoMap)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
					return
				}
				t.InfoBytes = infoBytes
				modified = true
			}
			if !modified {
				dialog.ShowInformation("No Changes", "No modifications were specified", w)
				return
			}
			f, err := os.Create(outputPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to create output file: %w", err), w)
				return
			}
			defer f.Close()
			if err := t.Write(f); err != nil {
				dialog.ShowError(fmt.Errorf("failed to write torrent: %w", err), w)
				return
			}
			dialog.ShowInformation("Torrent Modified", fmt.Sprintf("Successfully modified and saved to: %s", outputPath), w)
		}()
	})
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Torrent File", Widget: container.NewVBox(selectedFileLabel, selectButton)},
			{Text: "New Tracker URL", Widget: trackerEntry}, {Text: "Private", Widget: privateCheck},
			{Text: "Source", Widget: sourceEntry}, {Text: "Comment", Widget: commentEntry},
			{Text: "Output File", Widget: outputContainer}, {Text: "Randomize Hash", Widget: randomizeCheck},
		},
		SubmitText: "Modify Torrent", OnSubmit: func() { modifyButton.OnTapped() },
	}
	content := container.NewVBox(widget.NewLabelWithStyle("Modify a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), form)
	return container.NewPadded(content)
}
