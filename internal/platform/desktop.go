package platform

// 桌面相关：开机自启、打开文件夹、打开终端、系统深色模式。目前只实现了 Windows。

// BackgroundArg 开机自启时带的参数：不弹窗口，只在托盘里后台运行。
const BackgroundArg = "--background"

// AutostartCommand 开机自启要执行的命令。
func AutostartCommand(exe string) string { return `"` + exe + `" ` + BackgroundArg }

// Autostart 当前登记的开机自启命令（HKCU\...\Run），没有登记 ok=false。
func Autostart() (command string, ok bool) { return autostart() }

// SetAutostart 登记开机自启（当前用户，不需要管理员权限）。
func SetAutostart(exe string) error { return setAutostart(AutostartCommand(exe)) }

// RemoveAutostart 取消开机自启；本来就没有也算成功。
func RemoveAutostart() error { return removeAutostart() }

// OpenFolder 用资源管理器打开文件夹。
func OpenFolder(path string) error { return openFolder(path) }

// OpenTerminal 打开一个新的终端窗口，在 dir 里执行 command（执行完窗口留着）。
func OpenTerminal(dir string, command ...string) error { return openTerminal(dir, command) }

// DarkMode 系统应用是不是深色主题。
func DarkMode() bool { return darkMode() }
