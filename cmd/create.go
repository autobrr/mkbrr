package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/spf13/cobra"
)

var (
	trackerURL        string
	isPrivate         bool
	comment           string
	pieceLengthExp    *uint // for 2^n piece length, nil means automatic
	maxPieceLengthExp *uint // for maximum 2^n piece length, nil means no limit
	outputPath        string
	webSeeds          []string
	noDate            bool
	source            string
	verbose           bool
	batchFile         string
	presetName        string
	presetFile        string
)

var createCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new torrent file",
	Long: `Create a new torrent file from a file or directory.
Supports both single file/directory and batch mode using a YAML config file.
Supports presets for commonly used settings.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most one arg")
		}
		if len(args) == 0 && batchFile == "" {
			presetFlag := cmd.Flags().Lookup("preset")
			if presetFlag != nil && presetFlag.Changed {
				return fmt.Errorf("when using a preset (-P/--preset), you must provide a path to the content")
			}
			return fmt.Errorf("requires a path argument or --batch flag")
		}
		if len(args) == 1 && batchFile != "" {
			return fmt.Errorf("cannot specify both path argument and --batch flag")
		}
		return nil
	},
	RunE:                       runCreate,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// hide help flag
	createCmd.Flags().SortFlags = false
	createCmd.Flags().BoolP("help", "h", false, "help for create")
	if err := createCmd.Flags().MarkHidden("help"); err != nil {
		// This is initialization code, so we should panic
		panic(fmt.Errorf("failed to mark help flag as hidden: %w", err))
	}

	createCmd.Flags().StringVarP(&batchFile, "batch", "b", "", "batch config file (YAML)")
	createCmd.Flags().StringVarP(&presetName, "preset", "P", "", "use preset from config")
	createCmd.Flags().StringVar(&presetFile, "preset-file", "", "preset config file (default ~/.config/mkbrr/presets.yaml)")
	createCmd.Flags().StringVarP(&trackerURL, "tracker", "t", "", "tracker URL")
	createCmd.Flags().StringArrayVarP(&webSeeds, "web-seed", "w", nil, "add web seed URLs")
	createCmd.Flags().BoolVarP(&isPrivate, "private", "p", true, "make torrent private")
	createCmd.Flags().StringVarP(&comment, "comment", "c", "", "add comment")

	// piece length flag allows setting a fixed piece size of 2^n bytes
	// if not specified, piece length is calculated automatically based on total size
	var defaultPieceLength, defaultMaxPieceLength uint
	createCmd.Flags().UintVarP(&defaultPieceLength, "piece-length", "l", 0, "set piece length to 2^n bytes (16-27, automatic if not specified)")
	createCmd.Flags().UintVarP(&defaultMaxPieceLength, "max-piece-length", "m", 0, "limit maximum piece length to 2^n bytes (16-27, unlimited if not specified)")
	createCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("piece-length") {
			pieceLengthExp = &defaultPieceLength
		}
		if cmd.Flags().Changed("max-piece-length") {
			maxPieceLengthExp = &defaultMaxPieceLength
		}
	}

	createCmd.Flags().StringVarP(&outputPath, "output", "o", "", "set output path (default: <name>.torrent)")
	createCmd.Flags().StringVarP(&source, "source", "s", "", "add source string")
	createCmd.Flags().BoolVarP(&noDate, "no-date", "d", false, "don't write creation date")
	createCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "be verbose")

	createCmd.Flags().String("dev-cpuprofile", "", "write cpu profile to file (development flag)")

	createCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} /path/to/content [flags]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
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

	// batch mode
	if batchFile != "" {
		results, err := torrent.ProcessBatch(batchFile, verbose, version)
		if err != nil {
			return fmt.Errorf("batch processing failed: %w", err)
		}

		display := torrent.NewDisplay(torrent.NewFormatter(verbose))
		display.ShowBatchResults(results, time.Since(start))
		return nil
	}

	// get input path from args
	inputPath := args[0]

	// load preset if specified
	var opts torrent.CreateTorrentOptions
	if presetName != "" {
		// determine preset file path
		var presetFilePath string
		if presetFile != "" {
			// if preset file is explicitly specified, use that
			presetFilePath = presetFile
		} else {
			// check known locations in order
			locations := []string{
				presetFile,     // explicitly specified file
				"presets.yaml", // current directory
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

		// find preset file
		presetPath, err := preset.FindPresetFile(presetFilePath)
		if err != nil {
			return fmt.Errorf("could not find preset file: %w", err)
		}

		// load presets
		presets, err := preset.Load(presetPath)
		if err != nil {
			return fmt.Errorf("could not load presets: %w", err)
		}

		// get preset
		presetOpts, err := presets.GetPreset(presetName)
		if err != nil {
			return fmt.Errorf("could not get preset: %w", err)
		}

		// convert preset to options
		opts = torrent.CreateTorrentOptions{
			Path:       inputPath,
			TrackerURL: presetOpts.Trackers[0],
			WebSeeds:   presetOpts.WebSeeds,
			IsPrivate:  *presetOpts.Private,
			Comment:    presetOpts.Comment,
			Source:     presetOpts.Source,
			NoDate:     presetOpts.NoDate != nil && *presetOpts.NoDate,
			NoCreator:  presetOpts.NoCreator != nil && *presetOpts.NoCreator,
			Verbose:    verbose,
			Version:    version,
		}

		if presetOpts.PieceLength != 0 {
			pieceLen := presetOpts.PieceLength
			opts.PieceLengthExp = &pieceLen
		}

		if presetOpts.MaxPieceLength != 0 {
			maxPieceLen := presetOpts.MaxPieceLength
			opts.MaxPieceLength = &maxPieceLen
		}

		// override preset options with command line flags if specified
		if cmd.Flags().Changed("tracker") {
			opts.TrackerURL = trackerURL
		}
		if cmd.Flags().Changed("web-seed") {
			opts.WebSeeds = webSeeds
		}
		if cmd.Flags().Changed("private") {
			opts.IsPrivate = isPrivate
		}
		if cmd.Flags().Changed("comment") {
			opts.Comment = comment
		}
		if cmd.Flags().Changed("piece-length") {
			opts.PieceLengthExp = pieceLengthExp
		}
		if cmd.Flags().Changed("max-piece-length") {
			opts.MaxPieceLength = maxPieceLengthExp
		}
		if cmd.Flags().Changed("source") {
			opts.Source = source
		}
		if cmd.Flags().Changed("no-date") {
			opts.NoDate = noDate
		}
	} else {
		// use command line options
		opts = torrent.CreateTorrentOptions{
			Path:           inputPath,
			TrackerURL:     trackerURL,
			WebSeeds:       webSeeds,
			IsPrivate:      isPrivate,
			Comment:        comment,
			PieceLengthExp: pieceLengthExp,
			MaxPieceLength: maxPieceLengthExp,
			Source:         source,
			NoDate:         noDate,
			NoCreator:      false,
			Verbose:        verbose,
			Version:        version,
		}
	}

	// set output path if specified
	if outputPath != "" {
		opts.OutputPath = outputPath
	}

	// create torrent
	torrentInfo, err := torrent.Create(opts)
	if err != nil {
		return err
	}

	// display info
	display := torrent.NewDisplay(torrent.NewFormatter(verbose))
	display.ShowOutputPathWithTime(torrentInfo.Path, time.Since(start))

	return nil
}
