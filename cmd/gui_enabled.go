//go:build gui

package cmd

import "os"

// init adds commands specific to the GUI build.
func init() {
	rootCmd.AddCommand(guiCmd)
	rootCmd.AddCommand(versionCmd)
}

// runGuiIfNoArgs checks if no arguments were provided and runs the GUI command.
// This function is called from Execute() in root.go.
func runGuiIfNoArgs() bool {
	if len(os.Args) == 1 {
		// Find the gui command and execute it directly
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "gui" {
				// Execute the gui command's Run function
				// We pass nil for args as the gui command doesn't expect any
				cmd.Run(cmd, nil)
				return true // Indicate that the GUI was run
			}
		}
	}
	return false // Indicate GUI was not run
}
