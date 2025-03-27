package main

import (
	"os"
	"runtime/debug"

	"github.com/autobrr/mkbrr/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
		}
	}

	cmd.SetVersion(version, buildTime)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
