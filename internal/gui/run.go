// Package gui 图形界面：Wails 窗口（WebView2）+ 系统托盘 + 暴露给前端的接口（App）。
package gui

import (
	"context"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ALFO111111/claude-code-sync/internal/service"
)

// Run 打开主窗口，阻塞到退出。assets 是前端构建产物（frontend/dist）。
func Run(assets fs.FS) error {
	service.ApplyLanguage()
	app := NewApp()
	app.tray = newTray()
	app.tray.start(app)
	return wails.Run(&options.App{
		Title:     "claude-code-sync",
		Width:     1120,
		Height:    720,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 246, B: 248, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		// 点窗口的 X 不是退出，是最小化到托盘（托盘菜单里选"退出"才真退）
		OnBeforeClose: func(ctx context.Context) bool {
			if app.quitting.Load() {
				return false // 托盘退出：放行，真正关掉
			}
			wailsRuntime.WindowHide(ctx)
			return true
		},
		Bind: []any{app},
	})
}
