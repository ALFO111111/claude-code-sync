// Package platform 和操作系统打交道的部分：定时同步任务（Windows 计划任务 / macOS launchd）、
// 隐藏子进程窗口、开机自启、打开文件夹/终端、深色模式。
package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// TaskName 定时任务名。var 而非 const：测试里换成不存在的任务名来验证幂等删除。
var TaskName = "AgentSync"

// taskNames 本工具创建的所有定时任务：主任务（每小时）+ 登录时跑一次。
func taskNames() []string { return []string{TaskName, TaskName + "-Logon"} }

// legacyTaskNames 改名前（claude-code-sync）装的定时任务。var：测试里换成不存在的名字。
var legacyTaskNames = []string{"ClaudeCodeSync", "ClaudeCodeSync-Logon"}

// HideWindow 让子进程不弹黑窗口（Windows 定时任务/钩子里需要）。
func HideWindow(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		hideWindow(cmd)
	}
}

// ScheduleIntervalMin 定时任务的同步间隔（分钟）。安全网定位：正常同步靠 hooks 和文件监听，
// 这个只是防漏（对齐 Syncthing 默认 1 小时重扫的做法），不需要频繁。
const ScheduleIntervalMin = 60

// InstallSchedule 安装定时任务：每小时（和登录时）跑一次 `<exe> sync -q`。
// logonOK=false 表示"登录时跑一次"没装上（Windows 普通权限常不让建），不影响每小时那个。
func InstallSchedule(exe string) (logonOK bool, err error) { return installSchedule(exe) }

// ScheduledInfo 查定时任务：调用的 exe 路径、间隔分钟数；任务不存在时 ok=false。
func ScheduledInfo() (exe string, intervalMin int, ok bool) { return scheduledInfo() }

// RemoveSchedule 删除定时任务；任务本来就不存在也算成功。
func RemoveSchedule() error { return removeSchedule() }

// RemoveLegacySchedule 删除改名前装的定时任务（新名字的装好以后再调，免得中间一个都没有）；本来就没有也算成功。
func RemoveLegacySchedule() error { return removeLegacySchedule() }

// parseISODurationMin 解析 PowerShell 吐出来的间隔（"PT10M"、"PT1H"、"PT1H30M"），解析不了返回 0。
func parseISODurationMin(s string) int {
	s = strings.TrimPrefix(s, "PT")
	min := 0
	if i := strings.Index(s, "H"); i >= 0 {
		var h int
		if _, err := fmt.Sscanf(s[:i], "%d", &h); err == nil {
			min += h * 60
		}
		s = s[i+1:]
	}
	if i := strings.Index(s, "M"); i >= 0 {
		var m int
		if _, err := fmt.Sscanf(s[:i], "%d", &m); err == nil {
			min += m
		}
	}
	return min
}
