//go:build windows

package platform

import (
	"errors"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey        = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue      = "claude-code-sync"
	personalize   = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	lightThemeVal = "AppsUseLightTheme"
)

func autostart() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValue)
	return v, err == nil
}

func setAutostart(command string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(runValue, command)
}

func removeAutostart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func openFolder(path string) error { return exec.Command("explorer.exe", path).Start() }

// openTerminal 有 Windows Terminal 用它，没有就开 cmd；command 在 dir 里执行，执行完窗口不关。
func openTerminal(dir string, command []string) error {
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command(wt, append([]string{"-d", dir, "cmd", "/k"}, command...)...).Start()
	}
	cmd := exec.Command("cmd.exe", append([]string{"/c", "start", "", "/d", dir, "cmd", "/k"}, command...)...)
	hideWindow(cmd) // 只藏中转的那个 cmd，start 开出来的新窗口照常显示
	return cmd.Start()
}

func darkMode() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalize, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(lightThemeVal)
	return err == nil && v == 0
}
