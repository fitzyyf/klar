// 桌面窗口按 magpie：Go 用系统网页视图打开页面，页面和接口走同一个处理器。
// 标题栏不自己实现：顶栏那条拖拽层挂上 `--wails-draggable: drag`，Wails runtime
// 自己接 mousedown → `wails:drag`，双击走 `wails:drag:doubleclick`，动作按系统设置来。
package main

import (
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"klar.dev/desktop/internal/engine"
)

func main() {
	ui, err := uiDir()
	if err != nil {
		log.Fatal(err)
	}
	if ui == "" {
		log.Printf("页面 内嵌")
	} else {
		log.Printf("页面 %s", ui)
	}
	go watchSessions()
	desk := engine.New()

	app := application.New(application.Options{
		Name:        "Agent会话Review",
		Description: "按会话看调用链，通过或退回",
		Assets:      application.AssetOptions{Handler: handler(ui, desk)},
		Mac:         application.MacOptions{},
	})
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "Agent会话Review",
		URL:       "/",
		Width:     1440,
		Height:    900,
		MinWidth:  1100,
		MinHeight: 700,
		Hidden:    true,
		Mac: application.MacWindow{
			// 红绿灯留在页面顶栏上，顶栏自己标出哪一段能拖。
			TitleBar: application.MacTitleBarHiddenInset,
		},
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if runtime.GOOS == "linux" {
			log.Printf("Linux 仍用系统标题栏")
		}
		win.Show()
		win.Focus()
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}
