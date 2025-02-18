package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const banner = `         __   ___.                 
  _____ |  | _\_ |________________ 
 /     \|  |/ /| __ \_  __ \_  __ \
|  Y Y  \    < | \_\ \  | \/|  | \/
|__|_|  /__|_ \|___  /__|   |__|   
      \/     \/    \/              `

var (
	version   string
	buildTime string
)

var rootCmd = &cobra.Command{
	Use:   "mkbrr",
	Short: "A tool to inspect and create torrent files",
	Long:  banner + "\n\nmkbrr is a tool to create and inspect torrent files.",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
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

func Execute() error {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SilenceUsage = false

	rootCmd.AddCommand(versionCmd)

	rootCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [command]

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`)

	return rootCmd.Execute()
}
