package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"despacho/app/infrastructure/logging"
	"despacho/internal/desktop"
)

// The frontend is embedded in the binary. This is why package main lives at
// the module root and not under cmd/: //go:embed cannot reach paths with "..".
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger, closer, err := logging.Init()
	if err != nil {
		println("Error inicializando logs:", err.Error())
		return
	}
	defer closer.Close()

	services := desktop.New(logger)

	err = wails.Run(&options.App{
		Title:     "Despacho",
		Width:     1280,
		Height:    820,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 245, B: 247, A: 1},
		OnStartup:        services.OnStartup,
		Bind:             services.Bindings(),
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
