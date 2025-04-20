package guiapp

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"

	"github.com/autobrr/mkbrr/internal/preset"
)

func Run(version, buildTime, appName string) {
	a := app.NewWithID("com.autobrr.mkbrr")
	w := a.NewWindow(fmt.Sprintf("%s %s", appName, version))
	w.Resize(fyne.NewSize(800, 600))
	w.SetMaster()

	var presetConfig *preset.Config
	presetPath, err := preset.FindPresetFile("")
	if err == nil {
		presetConfig, err = preset.Load(presetPath)
		if err != nil {
			log.Printf("Error loading presets from %s: %v\n", presetPath, err)
		} else {
			log.Printf("Loaded presets from %s\n", presetPath)
		}
	} else {
		log.Println("No preset file found.")
	}

	createTab := createTorrentTab(w, version, appName, presetConfig)
	inspectTab := inspectTorrentTab(w)
	modifyTab := modifyTorrentTab(w, presetConfig)

	tabs := container.NewAppTabs(
		container.NewTabItem("Create", createTab),
		container.NewTabItem("Inspect", inspectTab),
		container.NewTabItem("Modify", modifyTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	w.SetContent(tabs)
	w.ShowAndRun()
}
