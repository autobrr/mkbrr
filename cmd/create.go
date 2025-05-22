package cmd

import (
	"fmt"
	"os"
	"runtime/pprof"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/autobrr/mkbrr/internal/trackers"
)

// createOptions encapsulates all command-line flag values for the create command
type createOptions struct {
	pieceLengthExp    *uint
	maxPieceLengthExp *uint
	trackerURL        string
	comment           string
	outputPath        string
	outputDir         string
	source            string
	batchFile         string
	presetName        string
	presetFile        string
	webSeeds          []string
	excludePatterns   []string
	includePatterns   []string
	createWorkers     int
	isPrivate         bool
	noDate            bool
	noCreator         bool
	verbose           bool
	entropy           bool
	quiet             bool
	skipPrefix        bool
}

var options = createOptions{
	isPrivate: true,
}

var createCmd = &cobra.Command{
	Use:   "create [path]",
	Short: "Create a new torrent file",
	Long: `Create a new torrent file from a file or directory.
Supports both single file/directory and batch mode using a YAML config file.
Supports presets for commonly used settings.
When a tracker URL is provided, the output filename will use the tracker domain (without TLD) as prefix by default (e.g. "example_filename.torrent"). This behavior can be disabled with --skip-prefix.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most one arg")
		}
		if len(args) == 0 && options.batchFile == "" {
			presetFlag := cmd.Flags().Lookup("preset")
			if presetFlag != nil && presetFlag.Changed {
				return fmt.Errorf("when using a preset (-P/--preset), you must provide a path to the content")
			}
			return fmt.Errorf("requires a path argument or --batch flag")
		}
		if len(args) == 1 && options.batchFile != "" {
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
	createCmd.Flags().SortFlags = false
	createCmd.Flags().StringVarP(&options.batchFile, "batch", "b", "", "batch config file (YAML)")

	createCmd.Flags().StringVarP(&options.presetName, "preset", "P", "", "use preset from config")
	createCmd.Flags().StringVar(&options.presetFile, "preset-file", "", "preset config file (default ~/.config/mkbrr/presets.yaml)")
	createCmd.Flags().StringVarP(&options.trackerURL, "tracker", "t", "", "tracker URL")
	createCmd.Flags().StringArrayVarP(&options.webSeeds, "web-seed", "w", nil, "add web seed URLs")
	createCmd.Flags().BoolVarP(&options.isPrivate, "private", "p", true, "make torrent private")
	createCmd.Flags().StringVarP(&options.comment, "comment", "c", "", "add comment")

	var defaultPieceLength, defaultMaxPieceLength uint
	createCmd.Flags().UintVarP(&defaultPieceLength, "piece-length", "l", 0, "set piece length to 2^n bytes (16-27, automatic if not specified)")
	createCmd.Flags().UintVarP(&defaultMaxPieceLength, "max-piece-length", "m", 0, "limit maximum piece length to 2^n bytes (16-27, unlimited if not specified)")
	createCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if cmd.Flags().Changed("piece-length") {
			options.pieceLengthExp = &defaultPieceLength
		}
		if cmd.Flags().Changed("max-piece-length") {
			options.maxPieceLengthExp = &defaultMaxPieceLength
		}
	}

	createCmd.Flags().StringVarP(&options.outputPath, "output", "o", "", "set output path (default: <name>.torrent)")
	createCmd.Flags().StringVar(&options.outputDir, "output-dir", "", "output directory for created torrent")
	createCmd.Flags().StringVarP(&options.source, "source", "s", "", "add source string")
	createCmd.Flags().BoolVarP(&options.noDate, "no-date", "d", false, "don't write creation date")
	createCmd.Flags().BoolVarP(&options.noCreator, "no-creator", "", false, "don't write creator")
	createCmd.Flags().BoolVarP(&options.entropy, "entropy", "e", false, "randomize info hash by adding entropy field")
	createCmd.Flags().BoolVarP(&options.verbose, "verbose", "v", false, "be verbose")
	createCmd.Flags().BoolVar(&options.quiet, "quiet", false, "reduced output mode (prints only final torrent path)")
	createCmd.Flags().BoolVarP(&options.skipPrefix, "skip-prefix", "", false, "don't add tracker domain prefix to output filename")
	createCmd.Flags().StringArrayVarP(&options.excludePatterns, "exclude", "", nil, "exclude files matching these patterns (e.g., \"*.nfo,*.jpg\" or --exclude \"*.nfo\" --exclude \"*.jpg\")")
	createCmd.Flags().StringArrayVarP(&options.includePatterns, "include", "", nil, "include only files matching these patterns (e.g., \"*.mkv,*.mp4\" or --include \"*.mkv\" --include \"*.mp4\")")
	createCmd.Flags().IntVar(&options.createWorkers, "workers", 0, "number of worker goroutines for hashing (0 for automatic)")

	createCmd.Flags().String("cpuprofile", "", "write cpu profile to file (development flag)")

	createCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} /path/to/content [flags]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// setupProfiling sets up CPU profiling if the --cpuprofile flag is set
// It returns a cleanup function that should be deferred by the caller
func setupProfiling(cmd *cobra.Command) (cleanup func(), err error) {
	cpuprofile, _ := cmd.Flags().GetString("cpuprofile")
	if cpuprofile == "" {
		return func() {}, nil
	}

	f, err := os.Create(cpuprofile)
	if err != nil {
		return nil, fmt.Errorf("could not create CPU profile: %w", err)
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("could not start CPU profile: %w", err)
	}

	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}, nil
}

// processBatchMode handles processing multiple torrents using a batch configuration file
func processBatchMode(opts createOptions, version string, startTime time.Time) error {
	results, err := torrent.ProcessBatch(opts.batchFile, opts.verbose, opts.quiet, version)
	if err != nil {
		return fmt.Errorf("batch processing failed: %w", err)
	}

	if opts.quiet {
		for _, result := range results {
			if result.Success {
				fmt.Println("Wrote:", result.Info.Path)
			}
		}
	} else {
		display := torrent.NewDisplay(torrent.NewFormatter(opts.verbose))
		display.ShowBatchResults(results, time.Since(startTime))
	}
	return nil
}

// buildCreateOptions creates a torrent.CreateTorrentOptions struct from command-line options and presets
func buildCreateOptions(cmd *cobra.Command, inputPath string, opts createOptions, version string) (torrent.CreateTorrentOptions, error) {
	createOpts := torrent.CreateTorrentOptions{
		Path:            inputPath,
		TrackerURL:      opts.trackerURL,
		WebSeeds:        opts.webSeeds,
		IsPrivate:       opts.isPrivate,
		Comment:         opts.comment,
		PieceLengthExp:  opts.pieceLengthExp,
		MaxPieceLength:  opts.maxPieceLengthExp,
		Source:          opts.source,
		NoDate:          opts.noDate,
		NoCreator:       opts.noCreator,
		Verbose:         opts.verbose,
		Version:         version,
		Entropy:         opts.entropy,
		Quiet:           opts.quiet,
		SkipPrefix:      opts.skipPrefix,
		ExcludePatterns: opts.excludePatterns,
		IncludePatterns: opts.includePatterns,
		Workers:         opts.createWorkers,
		OutputDir:       opts.outputDir,
	}

	// If a preset is specified, load the preset options and merge with command-line flags
	if opts.presetName != "" {
		presetFilePath, err := preset.FindPresetFile(opts.presetFile)
		if err != nil {
			return createOpts, fmt.Errorf("could not find preset file: %w", err)
		}

		presetOpts, err := preset.LoadPresetOptions(presetFilePath, opts.presetName)
		if err != nil {
			return createOpts, fmt.Errorf("could not load preset options: %w", err)
		}

		if len(presetOpts.Trackers) > 0 && !cmd.Flags().Changed("tracker") {
			createOpts.TrackerURL = presetOpts.Trackers[0]
		}

		if len(presetOpts.WebSeeds) > 0 && !cmd.Flags().Changed("web-seed") {
			createOpts.WebSeeds = presetOpts.WebSeeds
		}

		if presetOpts.Private != nil && !cmd.Flags().Changed("private") {
			createOpts.IsPrivate = *presetOpts.Private
		}

		if presetOpts.Comment != "" && !cmd.Flags().Changed("comment") {
			createOpts.Comment = presetOpts.Comment
		}

		if presetOpts.Source != "" && !cmd.Flags().Changed("source") {
			createOpts.Source = presetOpts.Source
		}

		if presetOpts.OutputDir != "" && !cmd.Flags().Changed("output-dir") {
			createOpts.OutputDir = presetOpts.OutputDir
		}

		if presetOpts.NoDate != nil && !cmd.Flags().Changed("no-date") {
			createOpts.NoDate = *presetOpts.NoDate
		}

		if presetOpts.NoCreator != nil && !cmd.Flags().Changed("no-creator") {
			createOpts.NoCreator = *presetOpts.NoCreator
		}

		if presetOpts.SkipPrefix != nil && !cmd.Flags().Changed("skip-prefix") {
			createOpts.SkipPrefix = *presetOpts.SkipPrefix
		}

		if presetOpts.PieceLength != 0 && !cmd.Flags().Changed("piece-length") {
			pieceLen := presetOpts.PieceLength
			createOpts.PieceLengthExp = &pieceLen
		}

		if presetOpts.MaxPieceLength != 0 && !cmd.Flags().Changed("max-piece-length") {
			maxPieceLen := presetOpts.MaxPieceLength
			createOpts.MaxPieceLength = &maxPieceLen
		}

		if !cmd.Flags().Changed("entropy") && presetOpts.Entropy != nil {
			createOpts.Entropy = *presetOpts.Entropy
		}

		if len(presetOpts.ExcludePatterns) > 0 {
			if !cmd.Flags().Changed("exclude") {
				createOpts.ExcludePatterns = slices.Clone(presetOpts.ExcludePatterns)
			} else {
				createOpts.ExcludePatterns = append(slices.Clone(presetOpts.ExcludePatterns), createOpts.ExcludePatterns...)
			}
		}

		if len(presetOpts.IncludePatterns) > 0 {
			if !cmd.Flags().Changed("include") {
				createOpts.IncludePatterns = slices.Clone(presetOpts.IncludePatterns)
			} else {
				createOpts.IncludePatterns = append(slices.Clone(presetOpts.IncludePatterns), createOpts.IncludePatterns...)
			}
		}
	}

	// Check for tracker's default source only if no source is set by flag or preset
	if createOpts.Source == "" && !cmd.Flags().Changed("source") {
		if trackerSource, ok := trackers.GetTrackerDefaultSource(createOpts.TrackerURL); ok {
			createOpts.Source = trackerSource
		}
	}

	if opts.outputPath != "" {
		createOpts.OutputPath = opts.outputPath
	}

	return createOpts, nil
}

// createSingleTorrent handles creating a single torrent file
func createSingleTorrent(cmd *cobra.Command, args []string, opts createOptions, version string, startTime time.Time) error {
	inputPath := args[0]

	createOpts, err := buildCreateOptions(cmd, inputPath, opts, version)
	if err != nil {
		return err
	}

	torrentInfo, err := torrent.Create(createOpts)
	if err != nil {
		return err
	}

	if opts.quiet {
		fmt.Println("Wrote:", torrentInfo.Path)
	} else {
		display := torrent.NewDisplay(torrent.NewFormatter(opts.verbose))
		display.ShowOutputPathWithTime(torrentInfo.Path, time.Since(startTime))
	}

	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	cleanup, err := setupProfiling(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	start := time.Now()

	if options.batchFile != "" {
		return processBatchMode(options, version, start)
	}

	return createSingleTorrent(cmd, args, options, version, start)
}
