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

func init() {
	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
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
}

// ExecuteCLI configures and executes the root command for CLI mode.
func ExecuteCLI() error {
	setupCommon() // Call setup first
	// Add commands that should be available in CLI mode
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(modifyCmd)
	rootCmd.AddCommand(updateCmd)
	return rootCmd.Execute()
}

// Execute remains for potential backward compatibility or internal use, defaulting to CLI.
// It's generally better to call ExecuteCLI directly.
func Execute() error {
	return ExecuteCLI()
}
