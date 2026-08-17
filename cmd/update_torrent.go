package cmd

import (
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/torrent"
)

type updateTorrentOptions struct {
	OutputPath      string
	InPlace         bool
	Force           bool
	RenamePairs     []string
	ExcludePatterns []string
	IncludePatterns []string
	Workers         int
	Verbose         bool
	Quiet           bool
}

var updateTorrentOpts updateTorrentOptions

var updateTorrentCmd = &cobra.Command{
	Use:   "update-torrent <torrent-file> <content-path>",
	Short: "Sync a v1 torrent after structural file changes",
	Long: `Structurally sync an existing v1 torrent with its content directory.

Piece hashes are reused when a piece maps to files assumed to be unchanged.
Same-path, same-size files are not read and are assumed to be byte-identical.
Removed files are dropped; new, resized, and unmapped renamed files are hashed.
Use --rename old=new only when a renamed file is known to be unchanged.

This command does not detect in-place edits that preserve file size. Recreate
the torrent for a full rehash when existing file bytes may have changed. Run
mkbrr check afterward when you need to validate the updated torrent against disk.

Discovery filters are not stored in torrent metadata. Repeat any --exclude or
--include values used by create. Trackers, creation date, creator, private/source
fields, and piece length are preserved from the input torrent.

By default, output is written beside the input with ".updated" before the
extension and must not already exist. Output must be outside the content
directory; this also applies to the input selected by --in-place. Use --output
to choose a destination or --in-place to replace the input atomically.
Multi-piece updates that reuse no hashes are rejected unless --force explicitly
permits them.`,
	Args:                  cobra.ExactArgs(2),
	RunE:                  runUpdateTorrent,
	DisableFlagsInUseLine: true,
	SilenceUsage:          true,
}

// init registers the update-torrent flags and usage template.
func init() {
	updateTorrentCmd.Flags().SortFlags = false
	updateTorrentCmd.Flags().StringVarP(&updateTorrentOpts.OutputPath, "output", "o", "", `output path (default: add ".updated" before the input extension)`)
	updateTorrentCmd.Flags().BoolVar(&updateTorrentOpts.InPlace, "in-place", false, "replace the input torrent atomically")
	updateTorrentCmd.Flags().BoolVar(&updateTorrentOpts.Force, "force", false, "allow a multi-piece update that reuses no hashes")
	updateTorrentCmd.Flags().StringArrayVar(&updateTorrentOpts.RenamePairs, "rename", nil, "map an old torrent path to a new path as old=new (repeatable)")
	updateTorrentCmd.Flags().StringArrayVar(&updateTorrentOpts.ExcludePatterns, "exclude", nil, "exclude files matching these patterns (comma-separated or repeatable)")
	updateTorrentCmd.Flags().StringArrayVar(&updateTorrentOpts.IncludePatterns, "include", nil, "include only files matching these patterns (comma-separated or repeatable)")
	updateTorrentCmd.Flags().IntVar(&updateTorrentOpts.Workers, "workers", 0, "number of worker goroutines for hashing (0 for automatic)")
	updateTorrentCmd.Flags().BoolVarP(&updateTorrentOpts.Verbose, "verbose", "v", false, "be verbose")
	updateTorrentCmd.Flags().BoolVarP(&updateTorrentOpts.Quiet, "quiet", "q", false, "print only the updated torrent path")
	updateTorrentCmd.MarkFlagsMutuallyExclusive("output", "in-place")
	updateTorrentCmd.MarkFlagsMutuallyExclusive("verbose", "quiet")

	updateTorrentCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} <torrent-file> <content-path> [flags]

Arguments:
  torrent-file   Existing v1 .torrent file
  content-path   File or directory containing the updated content

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

// runUpdateTorrent translates CLI arguments into the reusable torrent update API.
func runUpdateTorrent(_ *cobra.Command, args []string) error {
	renames, err := parseRenamePairs(updateTorrentOpts.RenamePairs)
	if err != nil {
		return err
	}

	result, err := torrent.UpdateTorrent(torrent.UpdateOptions{
		TorrentPath:     args[0],
		ContentPath:     args[1],
		OutputPath:      updateTorrentOpts.OutputPath,
		InPlace:         updateTorrentOpts.InPlace,
		Force:           updateTorrentOpts.Force,
		Renames:         renames,
		ExcludePatterns: updateTorrentOpts.ExcludePatterns,
		IncludePatterns: updateTorrentOpts.IncludePatterns,
		Workers:         updateTorrentOpts.Workers,
		Verbose:         updateTorrentOpts.Verbose,
		Quiet:           updateTorrentOpts.Quiet,
	})
	if err != nil {
		return err
	}

	if updateTorrentOpts.Quiet {
		fmt.Println(result.OutputPath)
		return nil
	}
	fmt.Printf("Updated %s: reused %d/%d pieces, hashed %d\n", result.OutputPath, result.ReusedPieces, result.TotalPieces, result.HashedPieces)
	return nil
}

// parseRenamePairs validates repeatable old=new path mappings.
func parseRenamePairs(pairs []string) (map[string]string, error) {
	renames := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		oldPath, newPath, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid rename %q: expected old=new", pair)
		}
		oldPath = normalizeRenamePath(oldPath)
		newPath = normalizeRenamePath(newPath)
		if oldPath == "" || newPath == "" {
			return nil, fmt.Errorf("invalid rename %q: expected old=new", pair)
		}
		if _, exists := renames[oldPath]; exists {
			return nil, fmt.Errorf("duplicate rename source %q", oldPath)
		}
		renames[oldPath] = newPath
	}
	return renames, nil
}

// normalizeRenamePath accepts only relative paths within the torrent content root.
func normalizeRenamePath(filePath string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if filePath == "" || strings.HasPrefix(filePath, "/") {
		return ""
	}
	cleaned := path.Clean(filePath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}
