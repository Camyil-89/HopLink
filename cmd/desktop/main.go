package main

import (
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	desktop "hoplink/internal/desktop/app"
	"hoplink/internal/desktop/app/assets"
)

var version = "dev"

func main() {
	desktop.DesktopVersion = version
	app := desktop.NewApp()
	err := wails.Run(&options.App{
		Title: fmt.Sprintf("HopLink %s", version), Width: 1600, Height: 1000, MinWidth: 1180, MinHeight: 760,
		AssetServer: &assetserver.Options{Assets: assets.Assets},
		OnStartup:   app.Startup,
		Bind:        []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
