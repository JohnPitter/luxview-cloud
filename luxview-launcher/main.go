package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "LuxView Cloud Games",
		Width:            1000,
		Height:           600,
		MinWidth:         1000,
		MinHeight:        600,
		Frameless:        true, // sem moldura nativa; controles próprios no titlebar
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 11, A: 1}, // zinc-950
		OnStartup:        app.startup,
		Bind:             []any{app},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
