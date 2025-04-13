package cmd

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:                   "update",
	Short:                 "Update mkbrr",
	Long:                  `Update mkbrr to latest version.`,
	RunE:                  runUpdate,
	DisableFlagsInUseLine: true,
}

func init() {
	updateCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}}
  
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	_, err := semver.ParseTolerant(version)
	if err != nil {
		return fmt.Errorf("could not parse version: %w", err)
	}

	latest, found, err := selfupdate.DetectLatest(cmd.Context(), selfupdate.ParseSlug("autobrr/mkbrr"))
	if err != nil {
		return fmt.Errorf("error occurred while detecting version: %w", err)
	}
	if !found {
		return fmt.Errorf("latest version for %s/%s could not be found from github repository", "autobrr/mkbrr", version)
	}

	if latest.LessOrEqual(version) {
		fmt.Printf("Current binary is the latest version: %s\n", version)
		return nil
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	if err := selfupdate.UpdateTo(cmd.Context(), latest.AssetURL, latest.AssetName, exe); err != nil {
		return fmt.Errorf("error occurred while updating binary: %w", err)
	}

	fmt.Printf("Successfully updated to version: %s\n", latest.Version())
	return nil
}
