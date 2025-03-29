package cmd

import (
	"github.com/spf13/cobra"
)

const banner = `         __   ___.                 
  _____ |  | _\_ |________________ 
 /     \|  |/ /| __ \_  __ \_  __ \
|  Y Y  \    < | \_\ \  | \/|  | \/
|__|_|  /__|_ \|___  /__|   |__|   
      \/     \/    \/              `

var rootCmd = &cobra.Command{
	Use:   "mkbrr",
	Short: "A tool to inspect and create torrent files",
	Long:  banner + "\n\nmkbrr is a tool to create and inspect torrent files.",
}

const commonUsageTemplate = `Usage:
  {{.CommandPath}} [command]

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`

// setupCommon prepares the rootCmd with common settings.
func setupCommon() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SilenceUsage = false
	rootCmd.SetUsageTemplate(commonUsageTemplate)
	rootCmd.AddCommand(versionCmd)
}

// ExecuteCLI configures and executes the root command for CLI mode.
func ExecuteCLI() error {
	setupCommon()
	return rootCmd.Execute()
}

// Execute remains for potential backward compatibility or internal use, defaulting to CLI.
func Execute() error {
	return ExecuteCLI()
}
