//go:build !windows && !darwin

package platform

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

var errUnsupported = errors.New("only Windows and macOS are supported for now")

func autostartCommand(exe string) string                    { return `"` + exe + `" ` + DaemonArg }
func autostart() (string, bool)                             { return "", false }
func setAutostart(string) error                             { return errUnsupported }
func removeAutostart() error                                { return nil }
func restartDaemon()                                        {}
func openFolder(path string) error                          { return exec.Command("xdg-open", path).Start() }
func openTerminal(dir string, command []string) error       { return errUnsupported }
func darkMode() bool                                        { return false }
func scheduledInfo() (exe string, intervalMin int, ok bool) { return "", 0, false }
func removeSchedule() error                                 { return nil }

func installSchedule(exe string) (bool, error) {
	return false, fmt.Errorf(i18n.T("这个系统还不支持自动安装定时同步，请用 cron: */%d * * * * %s sync -q"), ScheduleIntervalMin, exe)
}
