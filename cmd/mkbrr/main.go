package main

import (
	"os"

	"github.com/autobrr/mkbrr/cmd"
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

	cmd.SetVersion(version, buildTime)
	cmd.SetAppName("mkbrr")
	if err := cmd.ExecuteCLI(); err != nil {
		os.Exit(1)
	}
}
