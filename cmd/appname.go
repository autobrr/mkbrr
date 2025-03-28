package cmd

// GetAppName returns the application name determined by build tags.
// The actual value of appName is set in appname_cli.go or appname_gui.go.
func GetAppName() string {
	return appName
}
