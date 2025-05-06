package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/torrent"
)

// modifyOptions encapsulates command-line flag values for the modify command
type modifyOptions struct {
	PresetName string
	PresetFile string
	OutputDir  string
	Output     string
	Tracker    string
	Comment    string
	Source     string
	WebSeeds   []string
	DryRun     bool
	NoDate     bool
	NoCreator  bool
	Verbose    bool
	Quiet      bool
	SkipPrefix bool
	Private    bool
	Entropy    bool
}

var modifyOpts = modifyOptions{
	Private: true,
}

var modifyCmd = &cobra.Command{
	Use:   "modify [torrent files...]",
	Short: "Modify existing torrent files using a preset",
	Long: `Modify existing torrent files using a preset or flags.
This allows batch modification of torrent files with new tracker URLs, source tags, etc.
Original files are preserved and new files are created with the tracker domain (without TLD) as prefix, e.g. "example_filename.torrent".
A custom output filename can also be specified via --output.

Note: All unnecessary metadata will be stripped.`,
	Args:                  cobra.MinimumNArgs(1),
	RunE:                  runModify,
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
}

func init() {
	modifyCmd.Flags().SortFlags = false
	modifyCmd.Flags().StringVarP(&modifyOpts.PresetName, "preset", "P", "", "use preset from config")
	modifyCmd.Flags().StringVar(&modifyOpts.PresetFile, "preset-file", "", "preset config file (default: ~/.config/mkbrr/presets.yaml)")
	modifyCmd.Flags().StringVar(&modifyOpts.OutputDir, "output-dir", "", "output directory for modified files")
	modifyCmd.Flags().StringVarP(&modifyOpts.Output, "output", "o", "", "custom output filename (without extension)")
	modifyCmd.Flags().BoolVarP(&modifyOpts.NoDate, "no-date", "d", false, "don't update creation date")
	modifyCmd.Flags().BoolVarP(&modifyOpts.NoCreator, "no-creator", "", false, "don't write creator")
	modifyCmd.Flags().StringVarP(&modifyOpts.Tracker, "tracker", "t", "", "tracker URL")
	modifyCmd.Flags().StringArrayVarP(&modifyOpts.WebSeeds, "web-seed", "w", nil, "add web seed URLs")
	modifyCmd.Flags().BoolVarP(&modifyOpts.Private, "private", "p", true, "make torrent private (default: true)")
	modifyCmd.Flags().StringVarP(&modifyOpts.Comment, "comment", "c", "", "add comment")
	modifyCmd.Flags().StringVarP(&modifyOpts.Source, "source", "s", "", "add source string")
	modifyCmd.Flags().BoolVarP(&modifyOpts.Entropy, "entropy", "e", false, "randomize info hash by adding entropy field")
	modifyCmd.Flags().BoolVarP(&modifyOpts.Verbose, "verbose", "v", false, "be verbose")
	modifyCmd.Flags().BoolVar(&modifyOpts.Quiet, "quiet", false, "reduced output mode (prints only final torrent paths)")
	modifyCmd.Flags().BoolVarP(&modifyOpts.SkipPrefix, "skip-prefix", "", false, "don't add tracker domain prefix to output filename")
	modifyCmd.Flags().BoolVarP(&modifyOpts.DryRun, "dry-run", "n", false, "show what would be modified without making changes")

	modifyCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [flags] [torrent files...]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// buildTorrentOptions creates a torrent.Options struct from command-line flags
func buildTorrentOptions(cmd *cobra.Command, opts modifyOptions) torrent.Options {
	torrentOpts := torrent.Options{
		PresetName:    opts.PresetName,
		PresetFile:    opts.PresetFile,
		OutputDir:     opts.OutputDir,
		OutputPattern: opts.Output,
		NoDate:        opts.NoDate,
		NoCreator:     opts.NoCreator,
		DryRun:        opts.DryRun,
		Verbose:       opts.Verbose,
		Quiet:         opts.Quiet,
		TrackerURL:    opts.Tracker,
		WebSeeds:      opts.WebSeeds,
		Comment:       opts.Comment,
		Source:        opts.Source,
		Version:       version,
		Entropy:       opts.Entropy,
		SkipPrefix:    opts.SkipPrefix,
	}

	if cmd.Flags().Changed("private") {
		torrentOpts.IsPrivate = &opts.Private
	}

	return torrentOpts
}

// displayModifyResults handles showing the results of torrent modification
func displayModifyResults(results []*torrent.Result, opts modifyOptions, display *torrent.Display, startTime time.Time) int {
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

		if opts.Quiet {
			fmt.Println("Wrote:", result.OutputPath)
		} else {
			display.ShowOutputPathWithTime(result.OutputPath, time.Since(startTime))
		}
		successCount++
	}

	return successCount
}

func runModify(cmd *cobra.Command, args []string) error {
	start := time.Now()

	display := torrent.NewDisplay(torrent.NewFormatter(modifyOpts.Verbose))
	display.SetQuiet(modifyOpts.Quiet)
	display.ShowMessage(fmt.Sprintf("Modifying %d torrent files...", len(args)))

	// Build torrent options from command-line flags
	torrentOpts := buildTorrentOptions(cmd, modifyOpts)

	// Process the torrent files
	results, err := torrent.ProcessTorrents(args, torrentOpts)
	if err != nil {
		return fmt.Errorf("could not process torrent files: %w", err)
	}

	// Display the results
	displayModifyResults(results, modifyOpts, display, start)

	return nil
}
