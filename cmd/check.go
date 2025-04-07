package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/torrent"
)

var (
	checkVerbose bool
	checkQuiet   bool
)

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
	checkCmd.Flags().BoolP("help", "h", false, "help for check")
	checkCmd.Flags().BoolVarP(&checkVerbose, "verbose", "v", false, "show list of bad piece indices")
	checkCmd.Flags().BoolVar(&checkQuiet, "quiet", false, "reduced output mode (prints only completion percentage)")
	checkCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} <torrent-file> <content-path> [flags]

Arguments:
  torrent-file   Path to the .torrent file
  content-path   Path to the directory or file containing the data

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

func runCheck(cmd *cobra.Command, args []string) error {
	torrentPath := args[0]
	contentPath := args[1]

	if _, err := os.Stat(torrentPath); err != nil {
		return fmt.Errorf("invalid torrent file path %q: %w", torrentPath, err)
	}
	if _, err := os.Stat(contentPath); err != nil {
		return fmt.Errorf("invalid content path %q: %w", contentPath, err)
	}

	start := time.Now()

	opts := torrent.VerifyOptions{
		TorrentPath: torrentPath,
		ContentPath: contentPath,
		Verbose:     checkVerbose,
		Quiet:       checkQuiet,
	}

	display := torrent.NewDisplay(torrent.NewFormatter(checkVerbose))

	if !checkQuiet {
		green := color.New(color.FgGreen).SprintFunc()
		cyan := color.New(color.FgCyan).SprintFunc()
		fmt.Fprintf(os.Stdout, "\n%s\n", green("Verifying:"))
		fmt.Fprintf(os.Stdout, "  Torrent file: %s\n", cyan(torrentPath))
		fmt.Fprintf(os.Stdout, "  Content: %s\n", cyan(contentPath))
	}

	result, err := torrent.VerifyData(opts)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	display.SetQuiet(checkQuiet)

	if checkQuiet {
		fmt.Printf("%.2f%%\n", result.Completion)
	} else {
		duration := time.Since(start)
		display.ShowVerificationResult(result, duration)
	}

	if result.BadPieces > 0 || len(result.MissingFiles) > 0 {
		return fmt.Errorf("verification failed or incomplete")
	}

	return nil
}
