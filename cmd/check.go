package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/torrent"
)

// checkOptions encapsulates all the flags for the check command
type checkOptions struct {
	Verbose bool
	Quiet   bool
	Workers int
}

var checkOpts checkOptions

var checkCmd = &cobra.Command{
	Use:   "check <torrent-file> <content-path>",
	Short: "Verify the integrity of content against a torrent file",
	Long: `Checks if the data in the specified content path (file or directory) matches
the pieces defined in the torrent file. This is useful for verifying downloads
or checking data integrity after moving files.`,
	Args:                       cobra.ExactArgs(2),
	RunE:                       runCheck,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	checkCmd.Flags().SortFlags = false
	checkCmd.Flags().BoolVarP(&checkOpts.Verbose, "verbose", "v", false, "show list of bad piece indices")
	checkCmd.Flags().BoolVar(&checkOpts.Quiet, "quiet", false, "reduced output mode (prints only completion percentage)")
	checkCmd.Flags().IntVar(&checkOpts.Workers, "workers", 0, "number of worker goroutines for verification (0 for automatic)")
	checkCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} <torrent-file> <content-path> [flags]

Arguments:
  torrent-file   Path to the .torrent file
  content-path   Path to the directory or file containing the data

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// validateCheckArgs validates the command arguments and returns the paths
func validateCheckArgs(args []string) (torrentPath string, contentPath string, err error) {
	torrentPath = args[0]
	contentPath = args[1]

	if _, err := os.Stat(torrentPath); err != nil {
		return "", "", fmt.Errorf("invalid torrent file path %q: %w", torrentPath, err)
	}

	if _, err := os.Stat(contentPath); err != nil {
		return "", "", fmt.Errorf("invalid content path %q: %w", contentPath, err)
	}

	return torrentPath, contentPath, nil
}

// buildVerifyOptions creates the verification options from the command flags
func buildVerifyOptions(opts checkOptions, torrentPath, contentPath string) torrent.VerifyOptions {
	return torrent.VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		Verbose:     opts.Verbose,
		Quiet:       opts.Quiet,
		Workers:     opts.Workers,
	}
}

// displayCheckResults handles the display of verification results
func displayCheckResults(display *torrent.Display, result *torrent.VerificationResult, duration time.Duration, opts checkOptions) {
	display.SetQuiet(opts.Quiet)

	if opts.Quiet {
		fmt.Printf("%.2f%%\n", result.Completion)
	} else {
		display.ShowVerificationResult(result, duration)
	}
}

func runCheck(cmd *cobra.Command, args []string) error {
	torrentPath, contentPath, err := validateCheckArgs(args)
	if err != nil {
		return err
	}

	start := time.Now()

	verifyOpts := buildVerifyOptions(checkOpts, torrentPath, contentPath)
	display := torrent.NewDisplay(torrent.NewFormatter(checkOpts.Verbose))

	if !checkOpts.Quiet {
		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()
		fmt.Fprintf(os.Stdout, "\n%s\n", green("Verifying:"))
		fmt.Fprintf(os.Stdout, "  Torrent file: %s\n", cyan(torrentPath))
		fmt.Fprintf(os.Stdout, "  Content: %s\n", cyan(contentPath))
	}

	result, err := torrent.VerifyData(verifyOpts)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	duration := time.Since(start)
	displayCheckResults(display, result, duration, checkOpts)

	if result.BadPieces > 0 || len(result.MissingFiles) > 0 {
		return fmt.Errorf("verification failed or incomplete")
	}

	return nil
}
