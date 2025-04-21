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
