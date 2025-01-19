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
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var (
	// flags for create command
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
)

var createCmd = &cobra.Command{
	Use:   "create <path>",
	Short: "Create a new torrent file",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	// hide help flag
	createCmd.Flags().SortFlags = false
	createCmd.Flags().BoolP("help", "h", false, "help for create")
	createCmd.Flags().MarkHidden("help")

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

	// use custom output path or default to name.torrent
	out := outputPath
	if out == "" {
		out = name + ".torrent"
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

	// save the torrent file
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		return fmt.Errorf("error writing torrent file: %w", err)
	}

	// Always show minimal info
	info := mi.GetInfo()
	fmt.Printf("\n\nCreated %s\n", out)
	fmt.Printf("Size: %s\n", humanize.Bytes(uint64(info.TotalLength())))
	fmt.Printf("Hash: %s\n", mi.HashInfoBytes().String())

	// Show detailed information only when verbose flag is set
	if verbose {
		fmt.Printf("Name: %s\n", info.Name)
		fmt.Printf("Pieces: %d\n", len(info.Pieces)/20)
		fmt.Printf("Piece Length: %s\n", humanize.Bytes(uint64(info.PieceLength)))
		fmt.Printf("Private: %v\n", isPrivate)

		if trackerURL != "" {
			fmt.Printf("\nTracker: %s\n", trackerURL)
		}

		if len(webSeeds) > 0 {
			fmt.Println("\nWeb Seeds:")
			for _, seed := range webSeeds {
				fmt.Printf("  - %s\n", seed)
			}
		}

		// display creation info
		fmt.Printf("\nCreated by: %s\n", mi.CreatedBy)
		if !noDate {
			creationTime := time.Unix(mi.CreationDate, 0)
			fmt.Printf("Created: %s\n", creationTime.Format(time.RFC1123))
		}
		if comment != "" {
			fmt.Printf("Comment: %s\n", comment)
		}

		// generate and display magnet link
		magnet, _ := mi.MagnetV2()
		fmt.Printf("\nMagnet Link: %s\n", magnet)

		// display file information for multi-file torrents
		if len(info.Files) > 0 {
			fmt.Printf("\nFiles  %s\n", info.Name)

			// organize files by path components
			filesByPath := make(map[string][]torrent.FileEntry)
			for _, f := range info.Files {
				dir := filepath.Dir(strings.Join(f.Path, string(filepath.Separator)))
				if dir == "." {
					dir = ""
				}
				filesByPath[dir] = append(filesByPath[dir], torrent.FileEntry{
					Name: filepath.Base(strings.Join(f.Path, string(filepath.Separator))),
					Size: f.Length,
					Path: strings.Join(f.Path, string(filepath.Separator)),
				})
			}

			// print files in tree structure
			prefix := "       " // 7 spaces to align with "Files  "
			for dir, files := range filesByPath {
				if dir != "" {
					fmt.Printf("%s├─%s\n", prefix, dir)
					for i, file := range files {
						if i == len(files)-1 {
							fmt.Printf("%s│  └─%s [%s]\n", prefix, file.Name, humanize.Bytes(uint64(file.Size)))
						} else {
							fmt.Printf("%s│  ├─%s [%s]\n", prefix, file.Name, humanize.Bytes(uint64(file.Size)))
						}
					}
				} else {
					for i, file := range files {
						if i == len(files)-1 {
							fmt.Printf("%s└─%s [%s]\n", prefix, file.Name, humanize.Bytes(uint64(file.Size)))
						} else {
							fmt.Printf("%s├─%s [%s]\n", prefix, file.Name, humanize.Bytes(uint64(file.Size)))
						}
					}
				}
			}
		}
	}

	return nil
}
