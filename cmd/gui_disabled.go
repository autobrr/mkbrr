//go:build !gui

package cmd

// runGuiIfNoArgs is a placeholder for non-GUI builds.
// It always returns false, indicating the GUI was not run.
func runGuiIfNoArgs() bool {
	return false
}
