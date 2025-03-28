package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage" // Added for file filter
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/anacrolix/torrent/bencode"
	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Start the mkbrr GUI",
	Long:  "Start a graphical user interface for mkbrr",
	Run: func(cmd *cobra.Command, args []string) {
		runGUI()
	},
	DisableFlagsInUseLine: true,
}

func init() {
	rootCmd.AddCommand(guiCmd)
}

func runGUI() {
	a := app.NewWithID("com.autobrr.mkbrr")
	w := a.NewWindow("mkbrr")
	w.Resize(fyne.NewSize(800, 600))
	w.SetMaster()

	// Create tabs for different functionality
	createTab := createTorrentTab(w)
	inspectTab := inspectTorrentTab(w)
	modifyTab := modifyTorrentTab(w)

	// Create tab container with all tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Create", createTab),
		container.NewTabItem("Inspect", inspectTab),
		container.NewTabItem("Modify", modifyTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.ShowAndRun()
}

func createTorrentTab(w fyne.Window) fyne.CanvasObject {
	// File/directory selection
	selectedPathLabel := widget.NewLabel("No path selected")
	selectedPathLabel.Wrapping = fyne.TextWrapBreak

	// Button for selecting a directory
	selectDirButton := widget.NewButtonWithIcon("Select Directory", theme.FolderOpenIcon(), func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			selectedPathLabel.SetText(uri.Path())
		}, w)
		folderDialog.Resize(fyne.NewSize(700, 500)) // Set a larger size
		folderDialog.Show()
	})

	// Button for selecting a file
	selectFileButton := widget.NewButtonWithIcon("Select File", theme.FileIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			selectedPathLabel.SetText(uri.URI().Path())
		}, w)
		fileDialog.Resize(fyne.NewSize(700, 500)) // Set a larger size
		fileDialog.Show()
	})

	// Arrange buttons horizontally
	selectButtons := container.NewHBox(selectDirButton, selectFileButton)

	// Tracker URL input
	trackerEntry := widget.NewEntry()
	trackerEntry.SetPlaceHolder("https://example.com/announce")

	// Piece size selection
	pieceSizeOptions := []string{"Auto", "16 KiB", "32 KiB", "64 KiB", "128 KiB", "256 KiB", "512 KiB", "1 MiB", "2 MiB", "4 MiB", "8 MiB", "16 MiB"}
	pieceSizeSelect := widget.NewSelect(pieceSizeOptions, func(value string) {})
	pieceSizeSelect.SetSelectedIndex(0)

	// Private torrent checkbox
	privateCheck := widget.NewCheck("Private", nil)
	privateCheck.SetChecked(true)

	// Source input
	sourceEntry := widget.NewEntry()

	// Comment input
	commentEntry := widget.NewEntry()
	commentEntry.MultiLine = true
	commentEntry.Wrapping = fyne.TextWrapBreak

	// Output filename
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("auto-generated filename")

	// Browse button for output file location
	outputBrowseButton := widget.NewButton("Browse...", func() {
		saveDialog := dialog.NewFileSave(func(uri fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			outputEntry.SetText(uri.URI().Path())
		}, w)

		// Pre-populate filename
		currentOutput := outputEntry.Text
		if currentOutput == "" {
			inputPath := selectedPathLabel.Text
			if inputPath != "No path selected" {
				// Use the base name of the selected input path + .torrent as default
				currentOutput = filepath.Base(filepath.Clean(inputPath)) + ".torrent"
			} else {
				currentOutput = "new.torrent" // More generic default if no input selected yet
			}
		}
		// Set the filename in the dialog directly using the calculated default
		saveDialog.SetFileName(currentOutput)
		// Try setting initial directory (might not work perfectly on all OS)
		if dir := filepath.Dir(currentOutput); dir != "." && dir != "/" {
			// Attempt to list the directory URI
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

	// Create button
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
			// Auto-generate filename based on input path
			baseName := filepath.Base(path)
			outputPath = baseName + ".torrent"
			outputEntry.SetText(outputPath)
		}

		// Get piece size from selection
		var pieceSize uint
		switch pieceSizeSelect.Selected {
		case "Auto":
			pieceSize = 0 // Let the create function determine appropriate size
		case "16 KiB":
			pieceSize = 14 // 2^14 = 16 KiB
		case "32 KiB":
			pieceSize = 15 // 2^15 = 32 KiB
		case "64 KiB":
			pieceSize = 16 // 2^16 = 64 KiB
		case "128 KiB":
			pieceSize = 17 // 2^17 = 128 KiB
		case "256 KiB":
			pieceSize = 18 // 2^18 = 256 KiB
		case "512 KiB":
			pieceSize = 19 // 2^19 = 512 KiB
		case "1 MiB":
			pieceSize = 20 // 2^20 = 1 MiB
		case "2 MiB":
			pieceSize = 21 // 2^21 = 2 MiB
		case "4 MiB":
			pieceSize = 22 // 2^22 = 4 MiB
		case "8 MiB":
			pieceSize = 23 // 2^23 = 8 MiB
		case "16 MiB":
			pieceSize = 24 // 2^24 = 16 MiB
		default:
			pieceSize = 0
		}

		// Create progress dialog
		progress := dialog.NewProgress("Creating Torrent", "Processing files...", w)
		progress.Show()

		// Set up the options for creating a torrent
		isPrivate := privateCheck.Checked
		options := torrent.CreateTorrentOptions{
			Path:       path,
			OutputPath: outputPath,
			IsPrivate:  isPrivate,
			Source:     sourceEntry.Text,
			Comment:    commentEntry.Text,
			TrackerURL: trackerURL,
		}

		if pieceSize > 0 {
			options.PieceLengthExp = &pieceSize
		}

		// Create torrent in a goroutine to avoid blocking UI
		go func() {
			defer progress.Hide()

			// Create the torrent
			result, err := torrent.Create(options)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			// Show success dialog
			dialog.ShowInformation("Torrent Created",
				fmt.Sprintf("Successfully created torrent: %s\n\nInfo Hash: %s\nSize: %d bytes",
					outputPath, result.InfoHash, result.Size), w)
		}()
	})

	// Form layout
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Path", Widget: container.NewVBox(selectedPathLabel, selectButtons)}, // Use HBox container for buttons
			{Text: "Tracker URL", Widget: trackerEntry},
			{Text: "Piece Size", Widget: pieceSizeSelect},
			{Text: "Private", Widget: privateCheck},
			{Text: "Source", Widget: sourceEntry},
			{Text: "Comment", Widget: commentEntry},
			{Text: "Output File", Widget: outputContainer}, // Use container with browse button
		},
		SubmitText: "Create Torrent",
		OnSubmit: func() {
			createButton.OnTapped()
		},
	}

	return container.NewVBox(
		widget.NewLabelWithStyle("Create a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
	)
}

func inspectTorrentTab(w fyne.Window) fyne.CanvasObject {
	// File selection
	selectedFileLabel := widget.NewLabel("No torrent file selected")
	selectedFileLabel.Wrapping = fyne.TextWrapBreak

	// Create a button to select a torrent file
	selectButton := widget.NewButtonWithIcon("Select Torrent File", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			selectedFileLabel.SetText(uri.URI().Path())
		}, w)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"})) // Filter for .torrent files
		fileDialog.Resize(fyne.NewSize(700, 500))                                  // Set a larger size
		fileDialog.Show()
	})

	// Info display area
	infoText := widget.NewMultiLineEntry()
	infoText.Wrapping = fyne.TextWrapBreak
	infoText.SetPlaceHolder("Torrent information will be displayed here")
	infoText.Disable()

	// Inspect button
	inspectButton := widget.NewButtonWithIcon("Inspect Torrent", theme.SearchIcon(), func() {
		path := selectedFileLabel.Text
		if path == "No torrent file selected" {
			dialog.ShowError(fmt.Errorf("please select a torrent file"), w)
			return
		}

		// Show progress dialog
		progress := dialog.NewProgress("Inspecting Torrent", "Loading torrent data...", w)
		progress.Show()

		// Inspect the torrent in a goroutine
		go func() {
			defer progress.Hide()

			// Load the torrent
			t, err := torrent.LoadFromFile(path)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			info := t.GetInfo()

			// Build information display
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Name: %s\n", info.Name))
			sb.WriteString(fmt.Sprintf("Info Hash: %s\n", t.HashInfoBytes()))
			sb.WriteString(fmt.Sprintf("Created: %v\n", t.CreationDate))
			sb.WriteString(fmt.Sprintf("Piece Length: %d bytes\n", info.PieceLength))
			sb.WriteString(fmt.Sprintf("Pieces: %d\n", len(info.Pieces)/20))

			if info.Private != nil {
				sb.WriteString(fmt.Sprintf("Private: %t\n", *info.Private))
			}

			sb.WriteString(fmt.Sprintf("Total Size: %d bytes\n", info.TotalLength()))

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

			if len(info.Files) > 1 {
				sb.WriteString(fmt.Sprintf("\nFiles: %d\n", len(info.Files)))
				for _, file := range info.Files {
					filePath := filepath.Join(file.Path...)
					sb.WriteString(fmt.Sprintf("- %s (%d bytes)\n", filePath, file.Length))
				}
			}

			// Update the info display
			infoText.SetText(sb.String())
			infoText.Enable()
		}()
	})

	return container.NewVBox(
		widget.NewLabelWithStyle("Inspect a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewVBox(selectedFileLabel, selectButton),
		widget.NewSeparator(),
		inspectButton,
		container.NewScroll(infoText),
	)
}

func modifyTorrentTab(w fyne.Window) fyne.CanvasObject {
	// File selection
	selectedFileLabel := widget.NewLabel("No torrent file selected")
	selectedFileLabel.Wrapping = fyne.TextWrapBreak

	// Create a button to select a torrent file
	selectButton := widget.NewButtonWithIcon("Select Torrent File", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			selectedFileLabel.SetText(uri.URI().Path())
		}, w)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".torrent"})) // Filter for .torrent files
		fileDialog.Resize(fyne.NewSize(700, 500))                                  // Set a larger size
		fileDialog.Show()
	})

	// Tracker URL input
	trackerEntry := widget.NewEntry()
	trackerEntry.SetPlaceHolder("https://example.com/announce")

	// Private torrent checkbox
	privateCheck := widget.NewCheck("Private", nil)
	privateCheck.SetChecked(true)

	// Source input
	sourceEntry := widget.NewEntry()

	// Comment input
	commentEntry := widget.NewEntry()
	commentEntry.MultiLine = true
	commentEntry.Wrapping = fyne.TextWrapBreak

	// Output filename
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("Same as input (will overwrite)")

	// Browse button for output file location
	outputBrowseButton := widget.NewButton("Browse...", func() {
		saveDialog := dialog.NewFileSave(func(uri fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri == nil {
				return // User cancelled
			}
			outputEntry.SetText(uri.URI().Path())
		}, w)

		// Pre-populate filename
		currentOutput := outputEntry.Text
		if currentOutput == "" {
			inputPath := selectedFileLabel.Text
			if inputPath != "No torrent file selected" {
				currentOutput = inputPath // Default to overwriting input
			} else {
				currentOutput = "output.torrent" // Default if no input selected yet
			}
		}
		saveDialog.SetFileName(filepath.Base(currentOutput))
		// Try setting initial directory
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

	// Randomize info hash
	randomizeCheck := widget.NewCheck("Randomize Info Hash", nil)

	// Modify button
	modifyButton := widget.NewButtonWithIcon("Modify Torrent", theme.DocumentSaveIcon(), func() {
		path := selectedFileLabel.Text
		if path == "No torrent file selected" {
			dialog.ShowError(fmt.Errorf("please select a torrent file"), w)
			return
		}

		// Show progress dialog
		progress := dialog.NewProgress("Modifying Torrent", "Processing...", w)
		progress.Show()

		// Get output path
		outputPath := outputEntry.Text
		if outputPath == "" {
			outputPath = path
		}

		// Modify in a goroutine
		go func() {
			defer progress.Hide()

			// Load the torrent
			t, err := torrent.LoadFromFile(path)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}

			modified := false

			// Apply modifications
			if trackerEntry.Text != "" {
				t.Announce = trackerEntry.Text
				t.AnnounceList = [][]string{{trackerEntry.Text}}
				modified = true
			}

			// Update source if provided
			if sourceEntry.Text != "" {
				info := t.GetInfo()
				info.Source = sourceEntry.Text

				// Re-encode the info dictionary
				infoBytes, err := bencode.Marshal(info)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
					return
				}
				t.InfoBytes = infoBytes
				modified = true
			}

			// Update comment if provided
			if commentEntry.Text != "" {
				t.Comment = commentEntry.Text
				modified = true
			}

			// Update private flag
			isPrivate := privateCheck.Checked
			info := t.GetInfo()
			if (info.Private == nil) || (*info.Private != isPrivate) {
				info.Private = &isPrivate

				// Re-encode the info dictionary
				infoBytes, err := bencode.Marshal(info)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
					return
				}
				t.InfoBytes = infoBytes
				modified = true
			}

			// Randomize info hash if requested
			if randomizeCheck.Checked {
				// Add entropy field to randomize info hash
				infoMap := make(map[string]interface{})
				if err := bencode.Unmarshal(t.InfoBytes, &infoMap); err != nil {
					dialog.ShowError(fmt.Errorf("failed to decode info: %w", err), w)
					return
				}

				// Generate random string for entropy
				entropy, err := torrent.GenerateRandomString()
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to generate entropy: %w", err), w)
					return
				}

				infoMap["entropy"] = entropy

				// Re-encode the info map
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

			// Save the modified torrent
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

			dialog.ShowInformation("Torrent Modified",
				fmt.Sprintf("Successfully modified and saved to: %s", outputPath), w)
		}()
	})

	// Form layout
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Torrent File", Widget: container.NewVBox(selectedFileLabel, selectButton)},
			{Text: "New Tracker URL", Widget: trackerEntry},
			{Text: "Private", Widget: privateCheck},
			{Text: "Source", Widget: sourceEntry},
			{Text: "Comment", Widget: commentEntry},
			{Text: "Output File", Widget: outputContainer}, // Use container with browse button
			{Text: "Randomize Hash", Widget: randomizeCheck},
		},
		SubmitText: "Modify Torrent",
		OnSubmit: func() {
			modifyButton.OnTapped()
		},
	}

	return container.NewVBox(
		widget.NewLabelWithStyle("Modify a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		form,
	)
}
