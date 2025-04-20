package guiapp

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
)

func createTorrentTab(w fyne.Window, version, appName string, presetConfig *preset.Config) fyne.CanvasObject {
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
	noDateCheck := widget.NewCheck("Remove Creation Date", nil)
	noCreatorCheck := widget.NewCheck("Remove Creator", nil)
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

		defaultFilename := "new.torrent"
		inputPath := selectedPathLabel.Text
		if inputPath != "No path selected" {
			defaultFilename = filepath.Base(filepath.Clean(inputPath)) + ".torrent"
		}

		currentOutput := outputEntry.Text
		if currentOutput != "" {
			// If user already typed something, use that directory and filename
			saveDialog.SetFileName(filepath.Base(currentOutput))
			if dir := filepath.Dir(currentOutput); dir != "." && dir != "/" {
				listableURI, err := storage.ListerForURI(storage.NewFileURI(dir))
				if err == nil {
					saveDialog.SetLocation(listableURI)
				} else {
					log.Printf("Warning: Could not create URI for directory %s: %v", dir, err)
				}
			}
		} else {
			// User hasn't typed anything, determine default location
			saveDialog.SetFileName(defaultFilename)
			defaultDir, err := getDefaultSaveDirectory()
			if err == nil {
				listableURI, err := storage.ListerForURI(storage.NewFileURI(defaultDir))
				if err == nil {
					saveDialog.SetLocation(listableURI)
				} else {
					log.Printf("Warning: Could not create URI for default directory %s: %v", defaultDir, err)
				}
			} else {
				log.Printf("Warning: Could not get default save directory: %v", err)
			}
		}

		saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"}))
		saveDialog.Resize(fyne.NewSize(700, 500))
		saveDialog.Show()
	})
	outputContainer := container.NewBorder(nil, nil, nil, outputBrowseButton, outputEntry)
	randomizeCheck := widget.NewCheck("Randomize Info Hash", nil)

	workersEntry := widget.NewEntry()
	workersEntry.SetPlaceHolder("Choose number of workers or leave blank for automatic")

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

		// Handle NoDate and NoCreator options from preset
		if opts.NoDate != nil {
			noDateCheck.SetChecked(*opts.NoDate)
		}
		if opts.NoCreator != nil {
			noCreatorCheck.SetChecked(*opts.NoCreator)
		}
	})
	presetSelect.PlaceHolder = "Select Preset (Optional)"

	if presetConfig != nil {
		presetNames := []string{"Manual"}
		keys := make([]string, 0, len(presetConfig.Presets))
		for k := range presetConfig.Presets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		presetNames = append(presetNames, keys...)
		presetSelect.Options = presetNames
		presetSelect.SetSelectedIndex(0)
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
			baseName := filepath.Base(path) + ".torrent"
			defaultDir, err := getDefaultSaveDirectory()
			if err == nil {
				outputPath = filepath.Join(defaultDir, baseName)
			} else {
				log.Printf("Warning: Could not get default save directory: %v. Saving to current directory.", err)
				outputPath = baseName
			}
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
			NoDate:          noDateCheck.Checked,
			NoCreator:       noCreatorCheck.Checked,
		}

		if workersEntry.Text != "" {
			workers, err := strconv.Atoi(workersEntry.Text)
			if err != nil || workers < 0 {
				dialog.ShowError(fmt.Errorf("invalid workers value: must be a non-negative integer"), w)
				return
			}
			options.Workers = workers
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
			noDateCheck.SetChecked(false)
			noCreatorCheck.SetChecked(false)
			randomizeCheck.SetChecked(false)
			workersEntry.SetText("")
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
		&widget.FormItem{Text: "Remove Date", Widget: noDateCheck},
		&widget.FormItem{Text: "Remove Creator", Widget: noCreatorCheck},
		&widget.FormItem{Text: "Source", Widget: sourceEntry},
		&widget.FormItem{Text: "Comment", Widget: commentEntry},
		&widget.FormItem{Text: "Exclude Patterns", Widget: excludeEntry},
		&widget.FormItem{Text: "Include Patterns", Widget: includeEntry},
		&widget.FormItem{Text: "Output File", Widget: outputContainer},
		&widget.FormItem{Text: "Randomize Hash", Widget: randomizeCheck},
		&widget.FormItem{Text: "Workers", Widget: workersEntry},
	)

	form := &widget.Form{
		Items:      formItems,
		SubmitText: "Create Torrent",
		OnSubmit:   func() { createButton.OnTapped() },
	}

	content := container.NewVBox(widget.NewLabelWithStyle("Create a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), form)
	return container.NewPadded(content)
}
