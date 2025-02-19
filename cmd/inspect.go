package cmd

import (
	"fmt"
	"os"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/autobrr/mkbrr/internal/torrent"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	inspectVerbose bool
	cyan           = color.New(color.FgMagenta, color.Bold).SprintFunc()
	label          = color.New(color.Bold, color.FgHiWhite).SprintFunc()
)

var inspectCmd = &cobra.Command{
	Use:                        "inspect <torrent-file>",
	Short:                      "Inspect a torrent file",
	Long:                       "Inspect a torrent file",
	Args:                       cobra.ExactArgs(1),
	RunE:                       runInspect,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().BoolP("help", "h", false, "help for inspect")
	inspectCmd.Flags().BoolVarP(&inspectVerbose, "verbose", "v", false, "show all metadata fields")
	inspectCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} <torrent-file>

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

func runInspect(cmd *cobra.Command, args []string) error {
	rawBytes, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	mi, err := metainfo.LoadFromFile(args[0])
	if err != nil {
		return fmt.Errorf("error loading torrent: %w", err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return fmt.Errorf("error parsing info: %w", err)
	}

	t := &torrent.Torrent{MetaInfo: mi}
	display := torrent.NewDisplay(torrent.NewFormatter(true))
	display.ShowTorrentInfo(t, &info)

	if inspectVerbose {
		fmt.Printf("\n%s\n", cyan("Additional metadata:"))

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

	if info.IsDir() {
		display.ShowFileTree(&info)
	}

	return nil
}
