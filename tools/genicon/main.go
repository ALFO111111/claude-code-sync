package main

// 生成程序图标（图形见 internal/icon）：
//
//	build/windows/icon.ico      exe 图标（wails build 编进 exe，窗口、任务栏也用它）
//	build/appicon.png           1024px 原图（wails 的约定位置）
//	frontend/src/assets/logo.png 界面左上角 logo
//
// 托盘图标不在这里生成：托盘要多种状态（正常 / 同步中 / 出错），运行时现画。
// 用法：在仓库根目录 go run ./tools/genicon

import (
	"os"
	"path/filepath"

	"github.com/ALFO111111/agent-sync/internal/icon"
)

func main() {
	write("build/windows/icon.ico", icon.ICO(icon.AppSizes, icon.Style{}))
	write("build/appicon.png", icon.PNG(icon.Render(1024, icon.Style{})))
	write("frontend/src/assets/logo.png", icon.PNG(icon.Render(128, icon.Style{})))
}

func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}
