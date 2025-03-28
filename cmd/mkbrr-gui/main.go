package main

import (
	"os"

	"github.com/autobrr/mkbrr/cmd"
	"github.com/autobrr/mkbrr/guiapp"
)

var (
	version   string
	buildTime string
)

func main() {
	if version == "" {
		version = "dev"
	}
	if buildTime == "" {
		buildTime = "unknown"
	}

	// This logic allows the GUI build to handle specific command-line arguments
	// like 'version', 'update', or 'help' without fully invoking Cobra's
	// standard execution flow, which would pull in unwanted dependencies for the CLI build.
	runGui := true
	if len(os.Args) > 1 {
		firstArg := os.Args[1]
		cliCommands := map[string]bool{
			"version": true,
			"update":  true,
			"help":    true,
			"--help":  true,
			"-h":      true,
		}
		// If the first argument matches one of the designated CLI commands,
		// set runGui to false to trigger the CLI execution path.
		if cliCommands[firstArg] {
			runGui = false
		}
	}

	if runGui {
		guiapp.Run(version, buildTime, "mkbrr-gui")
	} else {
		cmd.SetVersion(version, buildTime)
		cmd.SetAppName("mkbrr-gui")
		if err := cmd.ExecuteCLI(); err != nil {
			os.Exit(1)
		}
	}
}
