package cmd

var appName = "mkbrr"

func SetAppName(name string) {
	appName = name
}

func GetAppName() string {
	return appName
}
