package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/spf13/cobra"
)

var (
	modifyPresetName string
	modifyPresetFile string
	modifyOutputDir  string
	modifyDryRun     bool
	modifyNoDate     bool
	modifyVerbose    bool
)

var modifyCmd = &cobra.Command{
	Use:   "modify [torrent files...]",
	Short: "Modify existing torrent files using a preset",
	Long: `Modify existing torrent files using a preset.
This allows batch modification of torrent files with new tracker URLs, source tags, etc.
Original files are preserved and new files are created with -[preset] or -modified suffix.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runModify,
}

func init() {
	rootCmd.AddCommand(modifyCmd)

	modifyCmd.Flags().SortFlags = false
	modifyCmd.Flags().BoolP("help", "h", false, "help for modify")
	if err := modifyCmd.Flags().MarkHidden("help"); err != nil {
		panic(fmt.Errorf("failed to mark help flag as hidden: %w", err))
	}

	modifyCmd.Flags().StringVarP(&modifyPresetName, "preset", "P", "", "use preset from config")
	modifyCmd.Flags().StringVar(&modifyPresetFile, "preset-file", "", "preset config file (default: ~/.config/mkbrr/presets.yaml)")
	modifyCmd.Flags().StringVar(&modifyOutputDir, "output-dir", "", "output directory for modified files")
	modifyCmd.Flags().BoolVarP(&modifyDryRun, "dry-run", "n", false, "show what would be modified without making changes")
	modifyCmd.Flags().BoolVarP(&modifyNoDate, "no-date", "d", false, "don't update creation date")
	modifyCmd.Flags().BoolVarP(&modifyVerbose, "verbose", "v", false, "be verbose")
}

func runModify(cmd *cobra.Command, args []string) error {
	start := time.Now()

	display := torrent.NewDisplay(torrent.NewFormatter(modifyVerbose))
	display.ShowMessage(fmt.Sprintf("Modifying %d torrent files...", len(args)))

	// load preset if specified
	var preset *torrent.PresetOpts
	if modifyPresetName != "" {
		// determine preset file path
		var presetFilePath string
		if modifyPresetFile != "" {
			presetFilePath = modifyPresetFile
		} else {
			// check known locations in order
			locations := []string{
				modifyPresetFile, // explicitly specified file
				"presets.yaml",   // current directory
			}

			// add user home directory locations
			if home, err := os.UserHomeDir(); err == nil {
				locations = append(locations,
					filepath.Join(home, ".config", "mkbrr", "presets.yaml"), // ~/.config/mkbrr/
					filepath.Join(home, ".mkbrr", "presets.yaml"),           // ~/.mkbrr/
				)
			}

			// find first existing preset file
			for _, loc := range locations {
				if _, err := os.Stat(loc); err == nil {
					presetFilePath = loc
					break
				}
			}

			if presetFilePath == "" {
				return fmt.Errorf("no preset file found in known locations, create one or specify with --preset-file")
			}
		}

		// load presets
		presets, err := torrent.LoadPresets(presetFilePath)
		if err != nil {
			return fmt.Errorf("failed to load presets from %s: %w", presetFilePath, err)
		}

		// get preset
		preset, err = presets.GetPreset(modifyPresetName)
		if err != nil {
			return fmt.Errorf("failed to get preset: %w", err)
		}
	}

	// process each torrent file
	successCount := 0
	for _, path := range args {
		// load torrent file
		mi, err := metainfo.LoadFromFile(path)
		if err != nil {
			display.ShowError(fmt.Sprintf("Error loading %s: %v", path, err))
			continue
		}

		// apply modifications
		wasModified := false

		info, err := mi.UnmarshalInfo()
		if err != nil {
			display.ShowError(fmt.Sprintf("Error unmarshaling info for %s: %v", path, err))
			continue
		}

		if preset != nil {
			// apply tracker URL
			if len(preset.Trackers) > 0 && (len(mi.AnnounceList) == 0 || mi.AnnounceList[0][0] != preset.Trackers[0]) {
				mi.Announce = preset.Trackers[0]
				mi.AnnounceList = [][]string{{preset.Trackers[0]}}
				wasModified = true
			}

			// apply source tag if info can be unmarshaled
			if preset.Source != "" && info.Source != preset.Source {
				info.Source = preset.Source
				wasModified = true
				// re-marshal the modified info
				if infoBytes, err := bencode.Marshal(info); err == nil {
					mi.InfoBytes = infoBytes
				}
			}

			// apply private flag
			isPrivate := info.Private != nil && *info.Private
			if isPrivate != preset.Private {
				info.Private = &preset.Private
				wasModified = true
				// re-marshal the modified info
				if infoBytes, err := bencode.Marshal(info); err == nil {
					mi.InfoBytes = infoBytes
				}
			}
		}

		// update creation date unless --no-date specified
		if !modifyNoDate {
			mi.CreationDate = time.Now().Unix()
			wasModified = true
		}

		if !wasModified {
			display.ShowMessage(fmt.Sprintf("Skipping %s (no changes needed)", path))
			continue
		}

		if modifyDryRun {
			display.ShowMessage(fmt.Sprintf("Would modify %s", path))
			continue
		}

		// determine output path
		dir := filepath.Dir(path)
		if modifyOutputDir != "" {
			dir = modifyOutputDir
		}

		base := filepath.Base(path)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)

		// add preset name or "modified" to filename
		suffix := "-modified"
		if modifyPresetName != "" {
			suffix = "-" + modifyPresetName
		}
		outPath := filepath.Join(dir, name+suffix+ext)

		// ensure output directory exists
		if err := os.MkdirAll(dir, 0755); err != nil {
			display.ShowError(fmt.Sprintf("Error creating output directory for %s: %v", outPath, err))
			continue
		}

		// save modified torrent
		f, err := os.Create(outPath)
		if err != nil {
			display.ShowError(fmt.Sprintf("Error creating %s: %v", outPath, err))
			continue
		}
		if err := mi.Write(f); err != nil {
			f.Close()
			display.ShowError(fmt.Sprintf("Error writing %s: %v", outPath, err))
			continue
		}
		f.Close()

		if modifyVerbose {
			display.ShowTorrentInfo(&torrent.Torrent{MetaInfo: mi}, &info)
		}
		display.ShowOutputPathWithTime(outPath, time.Since(start))
		successCount++
	}

	return nil
}
