package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/spf13/cobra"
)

var (
	trackerURL     string
	isPrivate      bool
	comment        string
	pieceLengthExp *uint // for 2^n piece length, nil means automatic
	outputPath     string
	torrentName    string
	webSeeds       []string
	noDate         bool
	source         string
	verbose        bool
	batchFile      string
)
var createCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new torrent file",
	Long: `Create a new torrent file from a file or directory.
Supports both single file/directory and batch mode using a YAML config file.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most one arg")
		}
		if len(args) == 0 && batchFile == "" {
			return fmt.Errorf("requires a path argument or --batch flag")
		}
		if len(args) == 1 && batchFile != "" {
			return fmt.Errorf("cannot specify both path argument and --batch flag")
		}
		return nil
	},
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// hide help flag
	createCmd.Flags().SortFlags = false
	createCmd.Flags().BoolP("help", "h", false, "help for create")
	createCmd.Flags().MarkHidden("help")

	createCmd.Flags().StringVarP(&batchFile, "batch", "b", "", "batch config file (YAML)")
	createCmd.Flags().StringVarP(&trackerURL, "tracker", "t", "", "tracker URL")
	createCmd.Flags().StringArrayVarP(&webSeeds, "web-seed", "w", nil, "add web seed URLs")
	createCmd.Flags().BoolVarP(&isPrivate, "private", "p", false, "make torrent private")
	createCmd.Flags().StringVarP(&comment, "comment", "c", "", "add comment")

	// piece length flag allows setting a fixed piece size of 2^n bytes
	// if not specified, piece length is calculated automatically based on total size
	var defaultPieceLength uint
	createCmd.Flags().UintVarP(&defaultPieceLength, "piece-length", "l", 0, "set piece length to 2^n bytes (14-24, automatic if not specified)")
	createCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("piece-length") {
			pieceLengthExp = &defaultPieceLength
		}
	}

	createCmd.Flags().StringVarP(&outputPath, "output", "o", "", "set output path (default: <n>.torrent)")
	createCmd.Flags().StringVarP(&torrentName, "name", "n", "", "set torrent name (default: basename of target)")
	createCmd.Flags().StringVarP(&source, "source", "s", "", "add source string")
	createCmd.Flags().BoolVarP(&noDate, "no-date", "d", false, "don't write creation date")
	createCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "be verbose")

	createCmd.Flags().String("cpuprofile", "", "write cpu profile to file")
}

func runCreate(cmd *cobra.Command, args []string) error {
	if cpuprofile, _ := cmd.Flags().GetString("cpuprofile"); cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
		defer pprof.StopCPUProfile()
	}

	start := time.Now()

	// Batch mode
	if batchFile != "" {
		results, err := torrent.ProcessBatch(batchFile, verbose, version)
		if err != nil {
			return fmt.Errorf("batch processing failed: %w", err)
		}

		display := torrent.NewDisplay(torrent.NewFormatter(verbose))
		display.ShowBatchResults(results, time.Since(start))
		return nil
	}

	// Single file mode
	if _, err := os.Stat(args[0]); err != nil {
		return fmt.Errorf("invalid path %q: %w", args[0], err)
	}

	if trackerURL != "" {
		if _, err := url.Parse(trackerURL); err != nil {
			return fmt.Errorf("invalid tracker URL %q: %w", trackerURL, err)
		}
	}

	for _, seed := range webSeeds {
		if _, err := url.Parse(seed); err != nil {
			return fmt.Errorf("invalid web seed URL %q: %w", seed, err)
		}
	}

	path := args[0]
	name := torrentName
	if name == "" {
		name = filepath.Base(filepath.Clean(path))
	}

	out := outputPath
	if out == "" {
		out = name + ".torrent"
	} else if !strings.HasSuffix(out, ".torrent") {
		out = out + ".torrent"
	}

	opts := torrent.CreateTorrentOptions{
		Path:           path,
		Name:           name,
		TrackerURL:     trackerURL,
		WebSeeds:       webSeeds,
		IsPrivate:      isPrivate,
		Comment:        comment,
		PieceLengthExp: pieceLengthExp,
		Source:         source,
		NoDate:         noDate,
		Verbose:        verbose,
		Version:        version,
	}

	mi, err := torrent.CreateTorrent(opts)
	if err != nil {
		return err
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		return fmt.Errorf("error writing torrent file: %w", err)
	}

	info := mi.GetInfo()
	display := torrent.NewDisplay(torrent.NewFormatter(verbose))

	display.ShowTorrentInfo(mi, info)

	if verbose && len(info.Files) > 0 {
		display.ShowFileTree(info)
	}

	display.ShowOutputPathWithTime(out, time.Since(start))

	return nil
}
