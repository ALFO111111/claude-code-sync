//go:build darwin

package platform

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/logging"
)

// macOS 上的定时同步和开机自启都用 launchd 的 LaunchAgent（~/Library/LaunchAgents/*.plist，当前用户，不需要管理员权限）：
//
//	com.alfo111111.claude-code-sync.sync     每小时 + 登录时跑一次 `<exe> sync -q`（兜底）
//	com.alfo111111.claude-code-sync.daemon   登录时启动 `<exe> daemon`：没有窗口，盯着文件变化随时同步
//
// Mac 版没有菜单栏图标（托盘库和 Wails 在 macOS 上会抢同一个应用主循环），所以"后台常驻"由 daemon 负责，
// 窗口就是普通窗口：关掉就退出，要看状态时再打开 App。
const launchdPrefix = "com.alfo111111.claude-code-sync"

var (
	syncLabel   = launchdPrefix + ".sync"
	daemonLabel = launchdPrefix + ".daemon"
)

type launchAgent struct {
	Label     string
	Args      []string
	Interval  int  // 秒；0 = 不定时
	KeepAlive bool // 异常退出时自动拉起
}

func agentPath(label string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (a launchAgent) plist() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + esc(a.Label) + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, arg := range a.Args {
		b.WriteString("\t\t<string>" + esc(arg) + "</string>\n")
	}
	b.WriteString("\t</array>\n\t<key>RunAtLoad</key>\n\t<true/>\n\t<key>ProcessType</key>\n\t<string>Background</string>\n")
	if a.Interval > 0 {
		b.WriteString("\t<key>StartInterval</key>\n\t<integer>" + strconv.Itoa(a.Interval) + "</integer>\n")
	}
	if a.KeepAlive {
		b.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	}
	b.WriteString("\t<key>StandardOutPath</key>\n\t<string>/dev/null</string>\n\t<key>StandardErrorPath</key>\n\t<string>/dev/null</string>\n</dict>\n</plist>\n")
	return b.Bytes()
}

var (
	reArgs     = regexp.MustCompile(`(?s)<key>ProgramArguments</key>\s*<array>(.*?)</array>`)
	reString   = regexp.MustCompile(`(?s)<string>(.*?)</string>`)
	reInterval = regexp.MustCompile(`<key>StartInterval</key>\s*<integer>(\d+)</integer>`)
)

// readAgent 读回本工具写的 plist（格式是自己写的，用正则取就够了）。
func readAgent(label string) (launchAgent, bool) {
	data, err := os.ReadFile(agentPath(label))
	if err != nil {
		return launchAgent{}, false
	}
	a := launchAgent{Label: label}
	if m := reArgs.FindSubmatch(data); m != nil {
		for _, s := range reString.FindAllSubmatch(m[1], -1) {
			a.Args = append(a.Args, html.UnescapeString(string(s[1])))
		}
	}
	if m := reInterval.FindSubmatch(data); m != nil {
		a.Interval, _ = strconv.Atoi(string(m[1]))
	}
	return a, len(a.Args) > 0
}

func launchctl(args ...string) ([]byte, error) {
	return exec.Command("launchctl", args...).CombinedOutput()
}

func guiDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

// writeAgent 写 plist 并让 launchd 立刻加载（不用等下次登录）。
// 加载失败（比如通过 SSH 登录、没有图形会话）不算失败：plist 已经写好，下次登录自动生效。
func writeAgent(a launchAgent) error {
	p := agentPath(a.Label)
	if err := fsutil.WriteFileAtomic(p, a.plist()); err != nil {
		return err
	}
	launchctl("bootout", guiDomain()+"/"+a.Label) // 没加载过会报错，忽略
	if out, err := launchctl("bootstrap", guiDomain(), p); err != nil {
		logging.New(true).Printf("launchctl 加载 %s 失败（下次登录时生效）: %v %s", a.Label, err, bytes.TrimSpace(out))
	}
	return nil
}

func removeAgent(label string) error {
	launchctl("bootout", guiDomain()+"/"+label)
	if err := os.Remove(agentPath(label)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func installSchedule(exe string) (logonOK bool, err error) {
	exe, err = filepath.Abs(exe)
	if err != nil {
		return false, err
	}
	// RunAtLoad：登录时也跑一次，相当于 Windows 的"登录时同步"
	return true, writeAgent(launchAgent{Label: syncLabel, Args: []string{exe, "sync", "-q"}, Interval: ScheduleIntervalMin * 60})
}

func scheduledInfo() (exe string, intervalMin int, ok bool) {
	a, ok := readAgent(syncLabel)
	if !ok {
		return "", 0, false
	}
	return a.Args[0], a.Interval / 60, true
}

func removeSchedule() error { return removeAgent(syncLabel) }
