package gui

import (
	"context"
	"errors"
	"os"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/logging"
	"github.com/ALFO111111/agent-sync/internal/service"
	"github.com/ALFO111111/agent-sync/internal/update"
)

// 检查更新（参照 CC Switch）：启动后查一次、之后每天一次，有新版本推 "update" 事件，侧边栏"设置"上挂红点；
// 设置页"关于"里手动检查、一键更新（下载 → 校验 → 替换 → 重启）。

// 自动检查的时机（测试可调）。
var (
	updateFirstCheck = 8 * time.Second // 启动后先让首页、首次同步跑起来
	updateInterval   = 24 * time.Hour
)

// UpdateState 设置页"关于"打开时要的全部状态。Info 是最近一次检查的结果（还没查过为 nil）。
type UpdateState struct {
	Current string       `json:"current"`
	Auto    bool         `json:"auto"`
	Skipped string       `json:"skipped"`
	Info    *update.Info `json:"info"`
}

// UpdateProgress 下载进度（事件 "update-progress"）。
type UpdateProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

func (a *App) autoCheckUpdate() {
	time.Sleep(updateFirstCheck)
	for {
		if service.GetUpdateSettings().Auto {
			if info, err := a.CheckUpdate(); err != nil {
				logging.New(true).Printf("检查更新失败: %v", err)
			} else if info.Available && info.Latest != service.GetUpdateSettings().Skipped {
				logging.New(true).Printf("发现新版本 %s（当前 %s）", info.Latest, info.Current)
			}
		}
		time.Sleep(updateInterval)
	}
}

func (a *App) GetUpdateState() UpdateState {
	s := service.GetUpdateSettings()
	a.updMu.Lock()
	defer a.updMu.Unlock()
	return UpdateState{Current: a.version, Auto: s.Auto, Skipped: s.Skipped, Info: a.updInfo}
}

// CheckUpdate 问 GitHub 有没有新版本，结果记下来并推 "update" 事件（侧边栏红点、设置页都跟着变）。
func (a *App) CheckUpdate() (*update.Info, error) {
	info, err := service.CheckUpdate(context.Background(), a.version)
	if err != nil {
		return nil, err
	}
	a.updMu.Lock()
	a.updInfo = info
	a.updMu.Unlock()
	a.emit("update", info)
	return info, nil
}

// InstallUpdate 下载新版本（推 "update-progress"），等正在跑的同步结束后替换、启动新版本、退出自己。
// 成功时程序随即退出，前端拿不到返回值；失败返回错误，原来的程序原样留着。
func (a *App) InstallUpdate() error {
	if !a.installing.CompareAndSwap(false, true) {
		return errors.New(i18n.T("正在更新，请稍候"))
	}
	defer a.installing.Store(false)
	a.updMu.Lock()
	info := a.updInfo
	a.updMu.Unlock()
	if info == nil || !info.Available {
		return errors.New(i18n.T("请先检查更新"))
	}
	locked := false
	exe, err := service.InstallUpdate(context.Background(), info,
		func(done, total int64) { a.emit("update-progress", UpdateProgress{done, total}) },
		func() {
			a.syncMu.Lock() // 替换前等这一轮同步跑完，之后也不再开新的同步
			locked = true
		})
	if err != nil {
		if locked {
			a.syncMu.Unlock()
		}
		logging.New(true).Printf("更新失败: %v", err)
		return err
	}
	logging.New(true).Printf("已更新到 %s，正在重新打开", info.Latest)
	if err := update.Relaunch(exe); err != nil {
		a.syncMu.Unlock()
		return err
	}
	a.quit()
	return nil
}

func (a *App) SkipUpdate(version string) error { return service.SkipUpdateVersion(version) }

func (a *App) SetAutoUpdateCheck(on bool) error { return service.SetAutoUpdateCheck(on) }

// quit 真正退出（托盘"退出"、更新完重启都走这里）。
func (a *App) quit() {
	a.quitting.Store(true) // 不然 OnBeforeClose 会把 Quit 当成"最小化到托盘"
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
	if a.tray != nil {
		a.tray.stop()
	}
	// 兜底：正常退出时 main 早就返回了，这个定时器不会触发；万一窗口关不掉
	// （比如 macOS 上要等下一个界面事件主循环才停），3 秒后强退，避免进程残留
	time.AfterFunc(3*time.Second, func() { os.Exit(0) })
}
