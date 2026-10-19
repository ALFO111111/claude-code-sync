// Package gui 图形界面：Wails 窗口（WebView2）+ 系统托盘 + 暴露给前端的接口（App）。
package gui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/platform"
	"github.com/ALFO111111/claude-code-sync/internal/service"
)

// BackgroundArg 开机自启时的命令行参数（见 platform.AutostartCommand）。
const BackgroundArg = platform.BackgroundArg

// Run 打开主窗口，阻塞到退出。assets 是前端构建产物（frontend/dist）。
// background=true（开机自启）时不弹窗口，只在托盘里运行，点托盘"打开主窗口"才显示。
func Run(assets fs.FS, background bool) error {
	service.ApplyLanguage()
	app := NewApp()
	app.tray = newTray()
	app.tray.start(app)
	bg := &options.RGBA{R: 245, G: 246, B: 248, A: 1}
	if service.GetTheme().Dark {
		bg = &options.RGBA{R: 24, G: 25, B: 28, A: 1} // 和前端深色主题的背景一致，打开时不闪白
	}
	return wails.Run(&options.App{
		Title:       "claude-code-sync",
		Width:       1120,
		Height:      720,
		MinWidth:    900,
		MinHeight:   600,
		StartHidden: background,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: bg,
		Windows:          &windows.Options{Theme: windowsTheme(service.GetTheme().Setting)},
		// 只允许开一个：开机自启已经在托盘里跑着时再双击 exe，就把那个窗口叫出来，不再开第二个
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID(),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.showWindow()
			},
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
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

// instanceID 单实例锁的名字。按数据目录区分：沙箱调试（CLAUDE_CODE_SYNC_HOME）的实例和平时用的互不影响。
func instanceID() string {
	h := sha256.Sum256([]byte(config.Dir()))
	return "claude-code-sync-" + hex.EncodeToString(h[:8])
}

// windowsTheme 标题栏颜色跟界面主题走。
func windowsTheme(setting string) windows.Theme {
	switch setting {
	case service.ThemeDark:
		return windows.Dark
	case service.ThemeLight:
		return windows.Light
	}
	return windows.SystemDefault
}
