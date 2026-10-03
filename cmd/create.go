// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/preset"
	"github.com/autobrr/mkbrr/torrent"
)

// createOptions encapsulates all command-line flag values for the create command
type createOptions struct {
	pieceLengthExp      uint
	maxPieceLengthExp   uint
	targetPieceCount    uint
	trackers            []string
	comment             string
	name                string
	outputPath          string
	outputDir           string
	source              string
	batchFile           string
	presetName          string
	presetFile          string
	webSeeds            []string
	excludePatterns     []string
	includePatterns     []string
	createWorkers       int
	isPrivate           bool
	noDate              bool
	noCreator           bool
	verbose             bool
	entropy             bool
	quiet               bool
	infoOnly            bool
	skipPrefix          bool
	failOnSeasonWarning bool
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
When a single tracker URL is provided, the output filename will use the tracker domain (without TLD) as prefix by default (e.g. "example_filename.torrent"). This behavior can be disabled with --skip-prefix. When multiple trackers are specified, no prefix is added.`,
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
	createCmd.Flags().StringArrayVarP(&options.trackers, "tracker", "t", nil, "tracker URLs (can be specified multiple times)")
	createCmd.Flags().StringArrayVarP(&options.webSeeds, "web-seed", "w", nil, "add web seed URLs")
	createCmd.Flags().BoolVarP(&options.isPrivate, "private", "p", true, "make torrent private")
	createCmd.Flags().StringVarP(&options.comment, "comment", "c", "", "add comment")

	createCmd.Flags().UintVarP(&options.pieceLengthExp, "piece-length", "l", 0, "set piece length to 2^n bytes (16-27, or 14-27 for a tracker with its own piece size table; automatic if not specified)")
	createCmd.Flags().UintVarP(&options.maxPieceLengthExp, "max-piece-length", "m", 0, "limit maximum piece length to 2^n bytes (16-27, unlimited if not specified)")
	createCmd.Flags().UintVar(&options.targetPieceCount, "target-piece-count", 0, "target approximate number of pieces (calculates optimal piece length)")

	createCmd.Flags().StringVar(&options.name, "name", "", "set torrent name (default: <filename>)")
	createCmd.Flags().StringVarP(&options.outputPath, "output", "o", "", "set output path (default: <filename>.torrent)")
	createCmd.Flags().StringVar(&options.outputDir, "output-dir", "", "output directory for created torrent")
	createCmd.Flags().StringVarP(&options.source, "source", "s", "", "add source string")
	createCmd.Flags().BoolVarP(&options.noDate, "no-date", "d", false, "don't write creation date")
	createCmd.Flags().BoolVarP(&options.noCreator, "no-creator", "", false, "don't write creator")
	createCmd.Flags().BoolVarP(&options.entropy, "entropy", "e", false, "randomize info hash by adding entropy field")
	createCmd.Flags().BoolVarP(&options.verbose, "verbose", "v", false, "be verbose")
	createCmd.Flags().BoolVarP(&options.quiet, "quiet", "q", false, "reduced output mode (prints only final torrent path)")
	createCmd.Flags().BoolVarP(&options.infoOnly, "info-only", "i", false, "display only torrent info without progress (implies verbose)")
	createCmd.Flags().BoolVarP(&options.skipPrefix, "skip-prefix", "", false, "don't add tracker domain prefix to output filename")
	createCmd.Flags().BoolVar(&options.failOnSeasonWarning, "fail-on-season-warning", false, "fail on season pack warning")
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
	results, err := torrent.ProcessBatch(opts.batchFile, opts.verbose, opts.quiet, opts.infoOnly, version)
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

// buildCreateOptions turns the changed command-line flags into create settings, loads the preset, and resolves the create options
func buildCreateOptions(cmd *cobra.Command, inputPath string, opts createOptions, version string) (torrent.CreateOptions, error) {
	flags := cmd.Flags()
	var s torrent.CreateSettings
	setIfChanged := func(name string, set func()) {
		if flags.Changed(name) {
			set()
		}
	}
	setIfChanged("piece-length", func() { s.PieceLengthExp = &opts.pieceLengthExp })
	setIfChanged("max-piece-length", func() { s.MaxPieceLength = &opts.maxPieceLengthExp })
	setIfChanged("target-piece-count", func() { s.TargetPieceCount = &opts.targetPieceCount })
	setIfChanged("tracker", func() { s.TrackerURLs = opts.trackers })
	setIfChanged("web-seed", func() { s.WebSeeds = opts.webSeeds })
	setIfChanged("exclude", func() { s.ExcludePatterns = opts.excludePatterns })
	setIfChanged("include", func() { s.IncludePatterns = opts.includePatterns })
	setIfChanged("private", func() { s.IsPrivate = &opts.isPrivate })
	setIfChanged("comment", func() { s.Comment = &opts.comment })
	setIfChanged("source", func() { s.Source = &opts.source })
	setIfChanged("output-dir", func() { s.OutputDir = &opts.outputDir })
	setIfChanged("no-date", func() { s.NoDate = &opts.noDate })
	setIfChanged("no-creator", func() { s.NoCreator = &opts.noCreator })
	setIfChanged("skip-prefix", func() { s.SkipPrefix = &opts.skipPrefix })
	setIfChanged("entropy", func() { s.Entropy = &opts.entropy })
	setIfChanged("fail-on-season-warning", func() { s.FailOnSeasonPackWarning = &opts.failOnSeasonWarning })
	setIfChanged("workers", func() { s.Workers = &opts.createWorkers })

	var presetOpts *preset.Options
	if opts.presetName != "" {
		presetFilePath, err := preset.FindPresetFile(opts.presetFile)
		if err != nil {
			return torrent.CreateOptions{}, fmt.Errorf("could not find preset file: %w", err)
		}

		presetOpts, err = preset.LoadPresetOptions(presetFilePath, opts.presetName)
		if err != nil {
			return torrent.CreateOptions{}, fmt.Errorf("could not load preset options: %w", err)
		}
	}

	return torrent.ResolveCreateOptions(torrent.CreateOverrides{
		CreateSettings: s,
		Path:           inputPath,
		Name:           opts.name,
		OutputPath:     opts.outputPath,
		Version:        version,
		Verbose:        opts.verbose,
		Quiet:          opts.quiet,
		InfoOnly:       opts.infoOnly,
	}, presetOpts)
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
	} else if !opts.infoOnly {
		display := torrent.NewDisplay(torrent.NewFormatter(opts.verbose))
		display.ShowOutputPathWithTime(torrentInfo.Path, time.Since(startTime))
	} else {
		if opts.infoOnly {
			prevNoColor := color.NoColor
			color.NoColor = true
			defer func() { color.NoColor = prevNoColor }()
		}
		display := torrent.NewDisplay(torrent.NewFormatter(opts.verbose || opts.infoOnly))
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
