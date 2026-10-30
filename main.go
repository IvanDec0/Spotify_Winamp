package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Frontend assets are embedded into the binary by Wails. The directory is
// produced by `npm run build` (or by `wails3 build`).
//
//go:embed all:frontend/dist
var assets embed.FS

const (
	appName = "Cadence"
	appID   = "io.github.ivandec0.cadence"
)

func main() {
	cadence := application.New(application.Options{
		Name:        appName,
		Description: "A compact Spotify Connect controller with declarative skins",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName:   "cadence",
			ApplicationID: appID,
		},
	})

	// One frameless window. Clarity's default size is 420x220; the queue
	// expands the window vertically while keeping the player visible.
	cadence.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            appName,
		Width:            420,
		Height:           220,
		MinWidth:         380,
		MinHeight:        200,
		MaxHeight:        560,
		Frameless:        true,
		BackgroundColour: application.NewRGB(9, 11, 15),
		URL:              "/",
		Linux: application.LinuxWindow{
			WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
		},
	})

	if err := cadence.Run(); err != nil {
		log.Fatal(err)
	}
}
