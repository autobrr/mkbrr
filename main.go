// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
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
		os.Exit(1)
	}
}
