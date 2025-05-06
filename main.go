package main

import (
	"fmt"
	"os"

	"github.com/autobrr/mkbrr/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.SetVersion(version, buildTime)
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
