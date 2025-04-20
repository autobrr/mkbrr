package guiapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/anacrolix/torrent/bencode"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
)

func modifyTorrentTab(w fyne.Window, presetConfig *preset.Config) fyne.CanvasObject {
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
	noDateCheck := widget.NewCheck("", nil)
	noCreatorCheck := widget.NewCheck("", nil)
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
	randomizeCheck := widget.NewCheck("", nil)

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

	modifyButton := widget.NewButtonWithIcon("Modify Torrent", theme.DocumentSaveIcon(), func() {
		path := selectedFileLabel.Text
		if path == "No torrent file selected" {
			dialog.ShowError(fmt.Errorf("please select a torrent file"), w)
			return
		}

		outputPath := outputEntry.Text
		if outputPath == "" {
			outputPath = path
		}

		clearFields := func() {
			selectedFileLabel.SetText("No torrent file selected")
			trackerEntry.SetText("")
			sourceEntry.SetText("")
			commentEntry.SetText("")
			outputEntry.SetText("")
			privateCheck.SetChecked(true)
			randomizeCheck.SetChecked(false)
			noDateCheck.SetChecked(false)
			noCreatorCheck.SetChecked(false)
			if presetConfig != nil {
				presetSelect.SetSelectedIndex(0)
			}
		}

		go func() {
			t, err := torrent.LoadFromFile(path)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			modified := false

			if presetConfig != nil && presetSelect.Selected != "" && presetSelect.Selected != "Manual" {
				opts, err := presetConfig.GetPreset(presetSelect.Selected)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to get preset '%s': %w", presetSelect.Selected, err), w)
					return
				}

				if len(opts.Trackers) > 0 {
					t.Announce = opts.Trackers[0]
					t.AnnounceList = [][]string{{opts.Trackers[0]}}
					modified = true
				}

				if opts.Source != "" || sourceEntry.Text == "" {
					info := t.GetInfo()
					info.Source = opts.Source
					infoBytes, err := bencode.Marshal(info)
					if err != nil {
						dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
						return
					}
					t.InfoBytes = infoBytes
					modified = true
				}

				if opts.Comment != "" || commentEntry.Text == "" {
					t.Comment = opts.Comment
					modified = true
				}

				if opts.Private != nil {
					info := t.GetInfo()
					if info.Private == nil || *info.Private != *opts.Private {
						info.Private = opts.Private
						infoBytes, err := bencode.Marshal(info)
						if err != nil {
							dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
							return
						}
						t.InfoBytes = infoBytes
						modified = true
					}
				}

				if opts.NoDate != nil && *opts.NoDate {
					t.CreationDate = 0
					modified = true
				}

				if opts.NoCreator != nil && *opts.NoCreator {
					t.CreatedBy = ""
					modified = true
				}
			}

			if trackerEntry.Text != "" {
				t.Announce = trackerEntry.Text
				t.AnnounceList = [][]string{{trackerEntry.Text}}
				modified = true
			}

			info := t.GetInfo()
			origSource := info.Source
			info.Source = sourceEntry.Text
			if info.Source != origSource {
				modified = true
			}
			infoBytes, err := bencode.Marshal(info)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to encode info: %w", err), w)
				return
			}
			t.InfoBytes = infoBytes

			origComment := t.Comment
			t.Comment = commentEntry.Text
			if t.Comment != origComment {
				modified = true
			}
			isPrivate := privateCheck.Checked
			info = t.GetInfo()
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

			if noDateCheck.Checked && t.CreationDate != 0 {
				t.CreationDate = 0
				modified = true
			}

			if noCreatorCheck.Checked && t.CreatedBy != "" {
				t.CreatedBy = ""
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

			successDialog := dialog.NewInformation("Torrent Modified", fmt.Sprintf("Successfully modified and saved to: %s", outputPath), w)
			successDialog.SetOnClosed(clearFields)
			successDialog.Show()
		}()
	})
	formItems := []*widget.FormItem{
		{Text: "Torrent File", Widget: container.NewVBox(selectedFileLabel, selectButton)},
	}

	if presetConfig != nil {
		formItems = append(formItems, &widget.FormItem{Text: "Preset", Widget: presetSelect})
	}

	formItems = append(formItems,
		&widget.FormItem{Text: "New Tracker URL", Widget: trackerEntry},
		&widget.FormItem{Text: "Private", Widget: privateCheck},
		&widget.FormItem{Text: "Source", Widget: sourceEntry},
		&widget.FormItem{Text: "Comment", Widget: commentEntry},
		&widget.FormItem{Text: "Remove Date", Widget: noDateCheck},
		&widget.FormItem{Text: "Remove Creator", Widget: noCreatorCheck},
		&widget.FormItem{Text: "Output File", Widget: outputContainer},
		&widget.FormItem{Text: "Randomize Hash", Widget: randomizeCheck},
	)

	form := &widget.Form{
		Items:      formItems,
		SubmitText: "Modify Torrent", OnSubmit: func() { modifyButton.OnTapped() },
	}
	content := container.NewVBox(widget.NewLabelWithStyle("Modify a Torrent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), form)
	return container.NewPadded(content)
}
