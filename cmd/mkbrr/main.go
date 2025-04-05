package main

import (
	"os"

	// Adjust import path relative to the new location
	"github.com/autobrr/mkbrr/cmd"
)

// These variables will be set by ldflags during the build process
// They must be declared in the main package for ldflags to work.
var (
	version   string
	buildTime string
)

func main() {
	// If ldflags didn't set the values, provide defaults.
	if version == "" {
		version = "dev"
	}
	if buildTime == "" {
		buildTime = "unknown"
	}

	// Pass the ldflags values (or defaults) to the cmd package
	cmd.SetVersion(version, buildTime)
	cmd.SetAppName("mkbrr") // Set app name for CLI
	if err := cmd.ExecuteCLI(); err != nil {
		os.Exit(1)
	}
}
