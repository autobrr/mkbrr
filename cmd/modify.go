package cmd

import (
	"fmt"
	"time"

	"github.com/autobrr/mkbrr/internal/modify"
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
	modifyTracker    string
	modifyWebSeeds   []string
	modifyPrivate    bool = true // default to true like create
	modifyComment    string
	modifySource     string
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
		panic(fmt.Errorf("could not mark help flag as hidden: %w", err))
	}

	modifyCmd.Flags().StringVarP(&modifyPresetName, "preset", "P", "", "use preset from config")
	modifyCmd.Flags().StringVar(&modifyPresetFile, "preset-file", "", "preset config file (default: ~/.config/mkbrr/presets.yaml)")
	modifyCmd.Flags().StringVar(&modifyOutputDir, "output-dir", "", "output directory for modified files")
	modifyCmd.Flags().BoolVarP(&modifyDryRun, "dry-run", "n", false, "show what would be modified without making changes")
	modifyCmd.Flags().BoolVarP(&modifyNoDate, "no-date", "d", false, "don't update creation date")
	modifyCmd.Flags().BoolVarP(&modifyVerbose, "verbose", "v", false, "be verbose")

	modifyCmd.Flags().StringVarP(&modifyTracker, "tracker", "t", "", "tracker URL")
	modifyCmd.Flags().StringArrayVarP(&modifyWebSeeds, "web-seed", "w", nil, "add web seed URLs")
	modifyCmd.Flags().BoolVarP(&modifyPrivate, "private", "p", true, "make torrent private (default: true)")
	modifyCmd.Flags().StringVarP(&modifyComment, "comment", "c", "", "add comment")
	modifyCmd.Flags().StringVarP(&modifySource, "source", "s", "", "add source string")
}

func runModify(cmd *cobra.Command, args []string) error {
	start := time.Now()

	display := torrent.NewDisplay(torrent.NewFormatter(modifyVerbose))
	display.ShowMessage(fmt.Sprintf("Modifying %d torrent files...", len(args)))

	// build opts, including our override flags defined above
	opts := modify.Options{
		PresetName: modifyPresetName,
		PresetFile: modifyPresetFile,
		OutputDir:  modifyOutputDir,
		NoDate:     modifyNoDate,
		DryRun:     modifyDryRun,
		Verbose:    modifyVerbose,
		TrackerURL: modifyTracker,
		WebSeeds:   modifyWebSeeds,
		Comment:    modifyComment,
		Source:     modifySource,
	}
	// always pass the private flag as pointer
	opts.IsPrivate = &modifyPrivate

	results, err := modify.ProcessTorrents(args, opts)
	if err != nil {
		return fmt.Errorf("could not process torrent files: %w", err)
	}

	// display results
	successCount := 0
	for _, result := range results {
		if result.Error != nil {
			display.ShowError(fmt.Sprintf("Error processing %s: %v", result.Path, result.Error))
			continue
		}

		if !result.WasModified {
			display.ShowMessage(fmt.Sprintf("Skipping %s (no changes needed)", result.Path))
			continue
		}

		if opts.DryRun {
			display.ShowMessage(fmt.Sprintf("Would modify %s", result.Path))
			continue
		}

		if opts.Verbose {
			// Load the modified torrent to display its info
			mi, err := torrent.LoadFromFile(result.OutputPath)
			if err == nil {
				info, err := mi.UnmarshalInfo()
				if err == nil {
					display.ShowTorrentInfo(mi, &info)
				}
			}
		}

		display.ShowOutputPathWithTime(result.OutputPath, time.Since(start))
		successCount++
	}

	return nil
}
