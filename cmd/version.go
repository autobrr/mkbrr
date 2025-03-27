package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version   string
	buildTime string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mkbrr version: %s\n", version)
		if buildTime != "unknown" {
			fmt.Printf("Build Time:    %s\n", buildTime)
		}
	},
	DisableFlagsInUseLine: true,
}

func SetVersion(v, bt string) {
	version = v
	buildTime = bt
}

func init() {
	versionCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}}

Prints the version and build time information for mkbrr.
`)
}
