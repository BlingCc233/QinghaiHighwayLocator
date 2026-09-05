package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "青海高速路产定位",
		Description: "西宁高速支队辖区路产定位与奥维对象导出",
		Services: []application.Service{
			application.NewService(&LocatorService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main-window",
		Title:     "青海高速路产定位",
		Width:     1180,
		Height:    820,
		MinWidth:  560,
		MinHeight: 620,
		MaxWidth:  2560,
		MaxHeight: 1800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(8, 15, 27),
		URL:              "/",
	})

	_ = app.Run()
}
