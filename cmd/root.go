package cmd

import (
	"os"

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
	// guiCmd is added in cmd/gui.go's init

	rootCmd.SetUsageTemplate(`Usage:
  {{.CommandPath}} [command]

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`)

	// If no arguments are provided (likely double-clicked), run the GUI command.
	if len(os.Args) == 1 {
		// Find the gui command and execute it directly
		// This assumes guiCmd is added to rootCmd in cmd/gui.go's init()
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "gui" {
				// Execute the gui command's Run function
				// We pass nil for args as the gui command doesn't expect any
				cmd.Run(cmd, nil)
				return nil // Exit after running the GUI
			}
		}
		// Fallback if gui command isn't found for some reason
		return rootCmd.Execute()
	}

	// Otherwise, execute normally using Cobra's argument parsing
	return rootCmd.Execute()
}
