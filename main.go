// claude-code-sync 把 Claude Code 的会话记录在多台 Windows 开发机之间同步。
//
// 单个 exe 两种模式：
//
//	无参数（双击）→ 图形界面  internal/gui（--background：开机自启，只在托盘里运行）
//	带参数        → 命令行    internal/cli（hooks、定时任务走这条路）
//
// 两边真正干活都调 internal/service；同步引擎在 internal/engine。
package main

import (
	"embed"
	"io/fs"
	"os"

	"github.com/ALFO111111/claude-code-sync/internal/cli"
	"github.com/ALFO111111/claude-code-sync/internal/gui"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

// version 发布时可用 -ldflags "-X main.version=x.y.z" 覆盖。
var version = "dev"

// 前端构建产物嵌进 exe，单文件分发（托盘图标运行时现画，见 internal/icon）。
// go:embed 只能引用本包目录及以下的文件，所以放在根目录的 main 包里。
//
//go:embed all:frontend/dist
var frontendDist embed.FS

func main() {
	if len(os.Args) < 2 || len(os.Args) == 2 && os.Args[1] == gui.BackgroundArg {
		assets, err := fs.Sub(frontendDist, "frontend/dist")
		if err == nil {
			err = gui.Run(assets, len(os.Args) == 2)
		}
		if err != nil {
			os.Stderr.WriteString(i18n.T("错误:") + " " + err.Error() + "\n")
			os.Exit(1)
		}
		return
	}
	os.Exit(cli.Run(os.Args[1:], version))
}
