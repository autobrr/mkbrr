package cmd

import (
	"fmt"
	"os"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/torrent"
)

// inspectOptions encapsulates command-line flag values for the inspect command
type inspectOptions struct {
	verbose bool
}

var (
	inspectOpts = inspectOptions{}
	cyan        = color.New(color.FgMagenta, color.Bold).SprintFunc()
	label       = color.New(color.Bold, color.FgHiWhite).SprintFunc()
)

var inspectCmd = &cobra.Command{
	Use:                        "inspect [flags] [torrent files...]",
	Short:                      "Inspect torrent files",
	Long:                       "Inspect torrent files",
	Args:                       cobra.MinimumNArgs(1),
	RunE:                       runInspect,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().BoolVarP(&inspectOpts.verbose, "verbose", "v", false, "show all metadata fields")
	inspectCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [flags] [torrent files...]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// loadTorrentData reads the torrent file and extracts metainfo, info, and raw bytes
func loadTorrentData(filePath string) (mi *metainfo.MetaInfo, info *metainfo.Info, rawBytes []byte, err error) {
	rawBytes, err = os.ReadFile(filePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading file: %w", err)
	}

	mi, err = metainfo.LoadFromFile(filePath)
	if err != nil {
		return nil, nil, rawBytes, fmt.Errorf("error loading torrent: %w", err)
	}

	parsedInfo, err := mi.UnmarshalInfo()
	if err != nil {
		return mi, nil, rawBytes, fmt.Errorf("error parsing info: %w", err)
	}

	return mi, &parsedInfo, rawBytes, nil
}

// displayStandardInfo shows the core information about the torrent
func displayStandardInfo(display *torrent.Display, mi *metainfo.MetaInfo, info *metainfo.Info) {
	t := &torrent.Torrent{MetaInfo: mi}
	display.ShowTorrentInfo(t, info)
}

// displayVerboseInfo shows additional metadata fields found in the torrent file
func displayVerboseInfo(rawBytes []byte, mi *metainfo.MetaInfo) {
	fmt.Printf("%s\n", cyan("Additional metadata:"))

	// Display extra root-level fields
	rootMap := make(map[string]interface{})
	if err := bencode.Unmarshal(rawBytes, &rootMap); err == nil {
		standardRoot := map[string]bool{
			"announce": true, "announce-list": true, "comment": true,
			"created by": true, "creation date": true, "info": true,
			"url-list": true, "nodes": true,
		}

		for k, v := range rootMap {
			if !standardRoot[k] {
				fmt.Printf("  %-13s %v\n", label(k+":"), v)
			}
		}
	}

	// Display extra info-dictionary fields
	infoMap := make(map[string]interface{})
	if err := bencode.Unmarshal(mi.InfoBytes, &infoMap); err == nil {
		standardInfo := map[string]bool{
			"name": true, "piece length": true, "pieces": true,
			"files": true, "length": true, "private": true,
			"source": true, "path": true, "paths": true,
			"md5sum": true,
		}

		for k, v := range infoMap {
			if !standardInfo[k] {
				fmt.Printf("  %-13s %v\n", label("info."+k+":"), v)
			}
		}
	}
	fmt.Println()
}

// displayFileTreeIfNeeded shows the file tree if the torrent contains multiple files
func displayFileTreeIfNeeded(display *torrent.Display, info *metainfo.Info) {
	if info.IsDir() {
		display.ShowFileTree(info)
	}
}

func runInspect(cmd *cobra.Command, args []string) error {
	display := torrent.NewDisplay(torrent.NewFormatter(inspectOpts.verbose))
	for _, path := range args {
		mi, info, rawBytes, err := loadTorrentData(path)
		if err != nil {
			return err
		}

		displayStandardInfo(display, mi, info)

		if inspectOpts.verbose {
			displayVerboseInfo(rawBytes, mi)
			displayFileTreeIfNeeded(display, info)
		}
	}

	return nil
}
