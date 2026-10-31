//go:build darwin

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 真的让 launchd 加载一个测试用的 LaunchAgent（标签带时间戳，跑的是 /usr/bin/true），再卸掉：
// 验证 plist 能被 launchd 接受、能读回 exe 和间隔、删除后不留痕迹。没有图形会话（CI）时只验证文件部分。
func TestLaunchAgentRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldSync, oldDaemon := syncLabel, daemonLabel
	suffix := fmt.Sprintf(".test%d", time.Now().UnixNano())
	syncLabel, daemonLabel = oldSync+suffix, oldDaemon+suffix
	defer func() { syncLabel, daemonLabel = oldSync, oldDaemon }()
	defer removeSchedule()
	defer removeAutostart()

	if _, err := InstallSchedule("/usr/bin/true"); err != nil {
		t.Fatal(err)
	}
	exe, interval, ok := ScheduledInfo()
	if !ok || exe != "/usr/bin/true" || interval != ScheduleIntervalMin {
		t.Fatalf("读回定时任务不对：%q %d %v", exe, interval, ok)
	}
	if out, err := launchctl("print", guiDomain()); err == nil {
		if out, err := launchctl("print", guiDomain()+"/"+syncLabel); err != nil {
			t.Fatalf("launchd 应已加载定时任务：%v %s", err, out)
		}
	} else {
		t.Logf("没有图形会话，跳过 launchd 加载检查：%s", out)
	}

	if err := SetAutostart("/usr/bin/true"); err != nil {
		t.Fatal(err)
	}
	if cmd, ok := Autostart(); !ok || cmd != AutostartCommand("/usr/bin/true") {
		t.Fatalf("读回开机自启不对：%q %v", cmd, ok)
	}

	if err := RemoveSchedule(); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAutostart(); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{syncLabel, daemonLabel} {
		if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", l+".plist")); !os.IsNotExist(err) {
			t.Errorf("%s 的 plist 应已删除", l)
		}
		if _, err := launchctl("print", guiDomain()+"/"+l); err == nil {
			t.Errorf("%s 卸载后不应还在 launchd 里", l)
		}
	}
	if err := RemoveSchedule(); err != nil {
		t.Fatal("重复删除不应报错")
	}
}

func TestPlistEscapesSpecialCharacters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := launchAgent{Label: "x.test", Args: []string{"/Applications/A & B <x>.app/Contents/MacOS/app", "sync"}}
	if err := os.MkdirAll(filepath.Dir(agentPath(a.Label)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentPath(a.Label), a.plist(), 0o600); err != nil {
		t.Fatal(err)
	}
	got, ok := readAgent(a.Label)
	if !ok || got.Args[0] != a.Args[0] || got.Args[1] != "sync" {
		t.Fatalf("特殊字符应原样读回：%+v", got)
	}
}
