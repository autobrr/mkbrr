// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/autobrr/mkbrr/internal/trackers"
	"github.com/autobrr/mkbrr/torrent"
)

type pieceSizeOptions struct {
	size    uint64
	tracker string
	json    bool
}

type pieceSizeResult struct {
	Exponent uint   `json:"exponent"`
	Bytes    uint64 `json:"bytes"`
	Human    string `json:"human"`
	Source   string `json:"source"`
}

var pieceSizeOpts = pieceSizeOptions{}

var pieceSizeCmd = &cobra.Command{
	Use:                        "piece-size",
	Short:                      "Calculate automatic piece size without creating a torrent",
	Long:                       "Calculate the piece size mkbrr would automatically select for a content size and optional tracker, without hashing files or creating a torrent.",
	Args:                       cobra.NoArgs,
	RunE:                       runPieceSize,
	DisableFlagsInUseLine:      true,
	SuggestionsMinimumDistance: 1,
	SilenceUsage:               true,
}

func init() {
	pieceSizeCmd.Flags().SortFlags = false
	pieceSizeCmd.Flags().Uint64Var(&pieceSizeOpts.size, "size", 0, "content size in bytes")
	pieceSizeCmd.Flags().StringVarP(&pieceSizeOpts.tracker, "tracker", "t", "", "tracker announce URL")
	pieceSizeCmd.Flags().BoolVar(&pieceSizeOpts.json, "json", false, "output machine-readable JSON")
	pieceSizeCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} --size <bytes> [flags]

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

func humanPieceSize(exp uint) string {
	bytes := uint64(1) << exp
	if bytes >= 1<<20 {
		return fmt.Sprintf("%d MiB", bytes>>20)
	}
	return fmt.Sprintf("%d KiB", bytes>>10)
}

func calculatePieceSizeResult(size uint64, trackerURL string) (pieceSizeResult, error) {
	if size == 0 {
		return pieceSizeResult{}, fmt.Errorf("content size must be greater than zero")
	}

	exp, err := torrent.GetAutomaticPieceLengthExp(trackerURL, size)
	if err != nil {
		return pieceSizeResult{}, err
	}

	source := "default"
	if trackerURL != "" {
		if _, ok := trackers.GetTrackerPieceSizeExp(trackerURL, size); ok {
			source = "tracker"
		} else if _, ok := trackers.GetTrackerMaxPieceLength(trackerURL); ok {
			source = "tracker"
		}
	}

	return pieceSizeResult{
		Exponent: exp,
		Bytes:    uint64(1) << exp,
		Human:    humanPieceSize(exp),
		Source:   source,
	}, nil
}

func runPieceSize(cmd *cobra.Command, _ []string) error {
	result, err := calculatePieceSizeResult(pieceSizeOpts.size, pieceSizeOpts.tracker)
	if err != nil {
		return err
	}

	if pieceSizeOpts.json {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Piece exponent: %d\n", result.Exponent)
	fmt.Fprintf(out, "Piece size: %s (%d bytes)\n", result.Human, result.Bytes)
	fmt.Fprintf(out, "Source: %s\n", result.Source)
	return nil
}
