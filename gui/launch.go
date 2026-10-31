// Copyright (c) 2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"path/filepath"
	"slices"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// launchCommands lists the commands that the GUI accepts on its command line,
// in the form "mkbrr-gui <command> [path]".
var launchCommands = []string{"inspect"}

// launchEvent tells the frontend that a new launch command is pending.
const launchEvent = "launch:command"

// LaunchCommand is a command from the command line that the frontend runs.
type LaunchCommand struct {
	Command string `json:"command"`
	Path    string `json:"path"` // Absolute, or empty when no path was given
}

// parseLaunchArgs returns the command in args, or nil when the first argument
// is not a known command. A relative path is resolved against workDir.
func parseLaunchArgs(args []string, workDir string) *LaunchCommand {
	if len(args) == 0 || !slices.Contains(launchCommands, args[0]) {
		return nil
	}
	cmd := &LaunchCommand{Command: args[0]}
	if len(args) > 1 && args[1] != "" {
		cmd.Path = args[1]
		if !filepath.IsAbs(cmd.Path) {
			cmd.Path = filepath.Join(workDir, cmd.Path)
		}
	}
	return cmd
}

// queueLaunch keeps the command in args until the frontend takes it.
// It reports whether args held a command.
func (a *App) queueLaunch(args []string, workDir string) bool {
	cmd := parseLaunchArgs(args, workDir)
	if cmd == nil {
		return false
	}
	a.mu.Lock()
	a.launch = cmd
	a.mu.Unlock()
	return true
}

// TakeLaunchCommand returns the pending launch command one time, or nil.
func (a *App) TakeLaunchCommand() *LaunchCommand {
	a.mu.Lock()
	defer a.mu.Unlock()
	cmd := a.launch
	a.launch = nil
	return cmd
}

// onSecondInstanceLaunch brings the open window to the front and gives it the
// command of the second launch.
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	queued := a.queueLaunch(data.Args, data.WorkingDirectory)

	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	// Before startup there is no window. The frontend takes the command on mount.
	if ctx == nil {
		return
	}
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	if queued {
		runtime.EventsEmit(ctx, launchEvent)
	}
}
