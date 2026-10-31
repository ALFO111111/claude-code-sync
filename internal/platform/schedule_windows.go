//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

// installSchedule 用 schtasks 而不是 PowerShell 的 Register-ScheduledTask：公司机器常限制任务目录权限，
// Register-ScheduledTask 会 Access denied（0x80070005），schtasks 走计划任务服务接口不受影响。
// "登录时也跑一次"的触发器（ONLOGON）很多机器普通权限不让建，尽力而为，失败不报错。
func installSchedule(exe string) (logonOK bool, err error) {
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

// scheduledInfo 读用 Get-ScheduledTask：普通权限也能读（写才需要权限）。
func scheduledInfo() (exe string, intervalMin int, ok bool) {
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

// removeSchedule 不靠报错文案判断"任务不存在"（中英文系统文案不同）：静默删除后逐个查一次，查不到就是目标状态已达成。
func removeSchedule() error {
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
