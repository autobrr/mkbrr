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

func Execute() error {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SilenceUsage = false

	// Add subcommands (versionCmd is added in its own file's init)
	// guiCmd is added conditionally in cmd/gui_enabled.go's init

	rootCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [command]

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`)

	// If no arguments are provided and this is a GUI build, run the GUI.
	// runGuiIfNoArgs() is defined in gui_enabled.go (for GUI builds)
	// or gui_disabled.go (for non-GUI builds).
	if runGuiIfNoArgs() {
		return nil // Exit after running the GUI
	}

	// Otherwise, execute normally using Cobra's argument parsing for CLI commands.
	return rootCmd.Execute()
}
