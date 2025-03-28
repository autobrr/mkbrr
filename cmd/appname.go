package cmd

// appName defines the application name, defaulting to "mkbrr"
var appName = "mkbrr"

// SetAppName sets the application name globally within the cmd package.
// This allows main_cli.go and main_gui.go to specify the correct name.
func SetAppName(name string) {
	appName = name
}

// GetAppName returns the currently set application name.
// This is used, for example, when creating the torrent file to set the "created by" field.
func GetAppName() string {
	return appName
}
