package gui

import (
	"math"
	"os"
	"sync"
	"time"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/icon"
	"github.com/ALFO111111/claude-code-sync/internal/service"
)

const (
	trayTitle    = "claude-code-sync"
	spinFrames   = 8 // 同步中动画：箭头图形转 180° 就和原来一样，所以半圈分 8 帧循环
	spinInterval = 120 * time.Millisecond
)

// tray 系统托盘：菜单（立即同步 / 打开主窗口 / 退出）+ 用图标和悬停提示显示同步状态：
//
//	平时     琥珀色循环箭头，悬停显示上次同步时间
//	同步中   箭头转圈
//	出错     右下角红点（下次同步成功后消失）
//
// 所有方法对 nil 安全（测试里的 App 没有托盘）。
type tray struct {
	idle, failed []byte
	spin         [][]byte

	mu       sync.Mutex
	ready    bool          // systray 初始化完成前不能调 SetIcon
	menu     []trayItem    // 菜单项，切换语言时改文字
	stopSpin chan struct{} // 转圈动画在跑时非 nil
	spinDone chan struct{}
}

func newTray() *tray {
	t := &tray{
		idle:   icon.ICO(icon.TraySizes, icon.Style{}),
		failed: icon.ICO(icon.TraySizes, icon.Style{Badge: true}),
	}
	for i := 0; i < spinFrames; i++ {
		t.spin = append(t.spin, icon.ICO(icon.TraySizes, icon.Style{Rotate: float64(i) * math.Pi / spinFrames}))
	}
	return t
}

// start 在后台跑 systray 消息循环。
func (t *tray) start(app *App) {
	go systray.Run(func() {
		t.mu.Lock()
		t.ready = true
		t.mu.Unlock()
		t.showStatus(service.LoadStatus())

		mSync := systray.AddMenuItem("", "")
		mOpen := systray.AddMenuItem("", "")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("", "")
		t.mu.Lock()
		t.menu = []trayItem{
			{mSync, func() string { return i18n.T("立即同步") }, func() string { return i18n.T("同步所有项目") }},
			{mOpen, func() string { return i18n.T("打开主窗口") }, func() string { return "" }},
			{mQuit, func() string { return i18n.T("退出") }, func() string { return i18n.T("退出 claude-code-sync") }},
		}
		t.mu.Unlock()
		t.relabel()
		go func() {
			for {
				select {
				case <-mSync.ClickedCh:
					app.SyncNow()
				case <-mOpen.ClickedCh:
					app.showWindow()
				case <-mQuit.ClickedCh:
					app.quitting.Store(true) // 不然 OnBeforeClose 会把 Quit 当成"最小化到托盘"
					if app.ctx != nil {
						wailsRuntime.Quit(app.ctx)
					}
					systray.Quit()
					// 兜底：正常退出时 main 早就返回了，这个定时器不会触发；
					// 万一窗口关不掉，3 秒后强退，避免进程残留
					time.AfterFunc(3*time.Second, func() { os.Exit(0) })
					return
				}
			}
		}()
	}, func() {})
}

// showStatus 按本机状态显示图标和提示（启动时用：上次同步有失败就挂红点）。
func (t *tray) showStatus(s *service.Status) {
	switch {
	case !s.Initialized:
		t.set(t.idle, tip(i18n.T("还没设置，打开主窗口按向导完成")))
	case s.KeyMismatch:
		t.set(t.failed, tip(i18n.T("同步已暂停：口令在其他设备上改了，打开主窗口输入新口令")))
	case s.Stats.Errors > 0:
		t.set(t.failed, tip(i18n.T("上次同步有失败，打开主窗口看日志")))
	case s.LastSync == "":
		t.set(t.idle, tip(i18n.T("还没同步过")))
	default:
		t.set(t.idle, tip(i18n.T("上次同步 %s", s.LastSync)))
	}
}

// trayItem 一个菜单项和它的文字（函数形式：切换语言后重新取）。
type trayItem struct {
	item           *systray.MenuItem
	title, tooltip func() string
}

// relabel 按当前语言刷新菜单文字。
func (t *tray) relabel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, m := range t.menu {
		m.item.SetTitle(m.title())
		m.item.SetTooltip(m.tooltip())
	}
}

// languageChanged 切换语言后刷新菜单和悬停提示（同步中不打断转圈，下一次换图标时提示自然更新）。
func (t *tray) languageChanged() {
	if t == nil {
		return
	}
	t.relabel()
	t.mu.Lock()
	spinning := t.stopSpin != nil
	t.mu.Unlock()
	if !spinning {
		t.showStatus(service.LoadStatus())
	}
}

// tip 悬停提示：第一行程序名，第二行状态。
func tip(status string) string { return trayTitle + "\n" + status }

// syncStarted 开始转圈。
func (t *tray) syncStarted() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready || t.stopSpin != nil {
		return
	}
	systray.SetTooltip(tip(i18n.T("同步中…")))
	stop, done := make(chan struct{}), make(chan struct{})
	t.stopSpin, t.spinDone = stop, done
	go func() {
		defer close(done)
		tk := time.NewTicker(spinInterval)
		defer tk.Stop()
		for i := 1; ; i++ {
			systray.SetIcon(t.spin[i%spinFrames])
			select {
			case <-stop:
				return
			case <-tk.C:
			}
		}
	}()
}

// syncFinished 停止转圈，按结果换图标。errMsg 为空表示成功。
func (t *tray) syncFinished(errMsg string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.stopSpin != nil {
		close(t.stopSpin)
		<-t.spinDone // 等动画真的停了再换图标，免得最后一帧盖掉结果
		t.stopSpin, t.spinDone = nil, nil
	}
	t.mu.Unlock()
	if errMsg != "" {
		t.set(t.failed, tip(i18n.T("同步失败：%s", truncateRunes(errMsg, 60))))
		return
	}
	t.set(t.idle, tip(i18n.T("上次同步 %s", time.Now().Format("2006-01-02 15:04"))))
}

func (t *tray) set(ico []byte, tip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		return
	}
	systray.SetIcon(ico)
	systray.SetTooltip(tip)
}

// truncateRunes 托盘提示最多 128 个字符（Windows 限制），按字符截断，不劈开汉字。
func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
