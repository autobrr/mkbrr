package cmd

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
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
	rootCmd.AddCommand(updateCmd)
	updateCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}}
  
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	v, err := semver.ParseTolerant(version)
	if err != nil {
		return fmt.Errorf("could not parse version: %w", err)
	}

	latest, err := selfupdate.UpdateSelf(v, "autobrr/mkbrr")
	if err != nil {
		return fmt.Errorf("could not selfupdate: %w", err)
	}

	if latest.Version.Equals(v) {
		// latest version is the same as current version. It means current binary is up-to-date.
		fmt.Printf("Current binary is the latest version: %s\n", version)
	} else {
		fmt.Printf("Successfully updated to version: %s\n", latest.Version)
	}
	return nil
}
