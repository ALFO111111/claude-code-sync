package platform

// 桌面相关：开机自启、打开文件夹、打开终端、系统深色模式。实现了 Windows 和 macOS。

// BackgroundArg Windows 开机自启时带的参数：不弹窗口，只在托盘里后台运行。
const BackgroundArg = "--background"

// DaemonArg macOS 开机自启运行的子命令：没有窗口，只盯文件变化做同步（Mac 版没有托盘，见 launchd_darwin.go）。
const DaemonArg = "daemon"

// RestartDaemon 更新完重启 macOS 的后台进程，让它跑新版本；没装（关了开机自启）或别的系统什么都不做。
func RestartDaemon() { restartDaemon() }

// AutostartCommand 开机自启要执行的命令（和 Autostart 返回的格式一致，用来判断登记的是不是当前 exe）。
func AutostartCommand(exe string) string { return autostartCommand(exe) }

// Autostart 当前登记的开机自启命令（Windows: HKCU\...\Run；macOS: LaunchAgent），没有登记 ok=false。
func Autostart() (command string, ok bool) { return autostart() }

// SetAutostart 登记开机自启（当前用户，不需要管理员权限）。
func SetAutostart(exe string) error { return setAutostart(exe) }

// RemoveAutostart 取消开机自启；本来就没有也算成功。
func RemoveAutostart() error { return removeAutostart() }

// RemoveLegacyAutostart 取消改名前登记的开机自启（Windows 注册表启动项 / macOS 后台进程）；本来就没有也算成功。
func RemoveLegacyAutostart() error { return removeLegacyAutostart() }

// OpenFolder 用资源管理器 / 访达打开文件夹。
func OpenFolder(path string) error { return openFolder(path) }

// OpenTerminal 打开一个新的终端窗口，在 dir 里执行 command（执行完窗口留着）。
func OpenTerminal(dir string, command ...string) error { return openTerminal(dir, command) }

// DarkMode 系统应用是不是深色主题。
func DarkMode() bool { return darkMode() }
