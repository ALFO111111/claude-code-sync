//go:build !windows

package gui

// macOS 暂时没有菜单栏图标：getlantern/systray 和 Wails 在 macOS 上都要接管同一个 NSApplication 主循环，
// 放在一起会冲突。后台常驻改由 `claude-code-sync daemon`（登录时 launchd 启动，没有窗口）负责盯文件变化，
// 主窗口就是普通窗口：关掉即退出。
type tray struct{}

func newTray() *tray { return &tray{} }

func (t *tray) start(*App)          {}
func (t *tray) syncStarted()        {}
func (t *tray) syncFinished(string) {}
func (t *tray) languageChanged()    {}
