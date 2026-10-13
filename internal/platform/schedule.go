// Package platform 和操作系统打交道的部分：Windows 定时任务、隐藏子进程窗口。
package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

// TaskName 定时任务名。var 而非 const：测试里换成不存在的任务名来验证幂等删除。
var TaskName = "ClaudeCodeSync"

// taskNames 本工具创建的所有定时任务：主任务（每 10 分钟）+ 登录时跑一次。
func taskNames() []string { return []string{TaskName, TaskName + "-Logon"} }

// HideWindow 让子进程不弹黑窗口（Windows 定时任务/钩子里需要）。
func HideWindow(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		hideWindow(cmd)
	}
}

// ScheduleIntervalMin 定时任务的同步间隔（分钟）。安全网定位：正常同步靠 hooks 和文件监听，
// 这个只是防漏（对齐 Syncthing 默认 1 小时重扫的做法），不需要频繁。
const ScheduleIntervalMin = 60

// InstallSchedule 安装定时任务：每小时跑一次 `<exe> sync -q`。
//
// 用 schtasks 而不是 PowerShell 的 Register-ScheduledTask：公司机器常限制任务目录权限，
// Register-ScheduledTask 会 Access denied（0x80070005），schtasks 走计划任务服务接口不受影响。
// "登录时也跑一次"的触发器（ONLOGON）很多机器普通权限不让建，尽力而为，失败不报错。
func InstallSchedule(exe string) (logonOK bool, err error) {
	if runtime.GOOS != "windows" {
		return false, fmt.Errorf(i18n.T("目前只支持 Windows 定时任务，其他系统请用 cron: */%d * * * * %s sync -q"), ScheduleIntervalMin, exe)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return false, err
	}
	run := `"` + exe + `" sync -q`
	create := func(name string, extra ...string) error {
		args := append([]string{"/Create", "/TN", name, "/TR", run, "/F"}, extra...)
		out, err := schtasks(args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf(i18n.T("安装定时任务 %s 失败: %v\n%s"), name, err, out)
		}
		return nil
	}
	if err := create(TaskName, "/SC", "HOURLY", "/MO", "1"); err != nil {
		return false, err
	}
	logonOK = create(TaskName+"-Logon", "/SC", "ONLOGON") == nil
	return logonOK, nil
}

// ScheduledInfo 查定时任务：调用的 exe 路径、间隔分钟数；任务不存在时 ok=false。
// 读用 Get-ScheduledTask：普通权限也能读（写才需要权限）。
func ScheduledInfo() (exe string, intervalMin int, ok bool) {
	if runtime.GOOS != "windows" {
		return "", 0, false
	}
	cmd := powershell(
		fmt.Sprintf(`$t = Get-ScheduledTask -TaskName '%s' -ErrorAction SilentlyContinue; `+
			`if ($t) { Write-Output $t.Actions[0].Execute; Write-Output $t.Triggers[0].Repetition.Interval } else { exit 1 }`, TaskName))
	out, err := cmd.Output()
	if err != nil {
		return "", 0, false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return "", 0, false
	}
	exe = strings.Trim(strings.TrimSpace(lines[0]), `"`) // schtasks 建的任务 Execute 可能带引号
	if len(lines) > 1 {
		intervalMin = parseISODurationMin(strings.TrimSpace(lines[1]))
	}
	return exe, intervalMin, true
}

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

// RemoveSchedule 删除定时任务；任务本来就不存在也算成功。
// 不靠报错文案判断"任务不存在"（中英文系统文案不同）：静默删除后逐个查一次，查不到就是目标状态已达成。
func RemoveSchedule() error {
	if runtime.GOOS != "windows" {
		return errors.New(i18n.T("非 Windows 系统请手动删除 cron 里的同步条目"))
	}
	for _, name := range taskNames() {
		schtasks("/Delete", "/TN", name, "/F").Run() // 不存在会报错，忽略
	}
	for _, name := range taskNames() {
		if schtasks("/Query", "/TN", name).Run() == nil {
			return fmt.Errorf(i18n.T("删除定时任务 %s 失败：删除后仍然存在"), name)
		}
	}
	return nil
}

// powershell 构造一条 PowerShell 命令（不弹黑窗口：GUI 进程里调用时尤其需要）。
func powershell(script string) *exec.Cmd {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	HideWindow(cmd)
	return cmd
}

// schtasks 构造一条 schtasks 命令（不弹黑窗口）。
func schtasks(args ...string) *exec.Cmd {
	cmd := exec.Command("schtasks", args...)
	HideWindow(cmd)
	return cmd
}
