package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/logging"
	"github.com/ALFO111111/claude-code-sync/internal/platform"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// 自动同步是程序自带的能力，不让用户手动开关：
//   Claude Code hooks（主力）：SessionStart / Stop 时调 "<exe> hook-event"，只同步当前项目
//   Windows 定时任务（兜底）：每小时 + 登录时全量同步
//   开机自启（GUI 在托盘里常驻，文件一变就同步）：默认开，设置页可以关
// 初始化完成时、每次打开图形界面时都会调 EnsureAutoSync：没装就装，exe 换了位置就把路径改过来。
// hooks 还可能被别的工具抹掉（CC Switch 切换供应商会重写 settings.json），见 RepairHooks。

var hookEvents = []string{"SessionStart", "Stop"}

// HookCommand 写进 settings.json 的 hook 命令。
func HookCommand() string { return `"` + ExePath() + `" hook-event` }

// isOurHook 判断一条 hook 命令是不是本工具装的（"<任意路径>" hook-event）。
func isOurHook(cmd string) bool { return strings.HasSuffix(strings.TrimSpace(cmd), `" hook-event`) }

// EnsureAutoSync 确保 hooks 和定时任务都已安装并指向当前 exe。
// 沙箱模式（设置了 CLAUDE_CODE_SYNC_HOME）什么都不做。失败会写进 sync.log。
func EnsureAutoSync() error {
	if config.Sandboxed() {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var errs []error
	if err := withSettingsLock(func() error { return ensureHooks(cfg.ClaudeDir, HookCommand()) }); err != nil {
		errs = append(errs, fmt.Errorf(i18n.T("安装 Claude Code hooks 失败: %w"), err))
	}
	if !scheduleCurrent() {
		logonOK, err := platform.InstallSchedule(ExePath())
		if err != nil {
			errs = append(errs, err)
		} else if !logonOK {
			// 登录触发器很多机器不让普通权限建，不影响主任务，记一笔日志就行
			logging.New(true).Printf("登录时自动同步的任务没装上（权限不够），每小时一次的定时同步不受影响")
		}
	}
	if err := syncAutostart(cfg); err != nil {
		errs = append(errs, err)
	}
	err = errors.Join(errs...)
	if err != nil {
		logging.New(true).Printf("自动同步安装失败: %v", err)
	}
	return err
}

// RepairHooks 只检查 hooks，不见了就补回（不碰定时任务，开销小，可以频繁调）。
// 为什么需要：CC Switch 这类工具切换供应商时会重写 ~/.claude/settings.json，
// 可能把我们的 hooks 抹掉，主力同步就悄悄停了。GUI 开着时盯着这个文件一变就调；
// GUI 没开时，每小时的定时任务也会调一次。settings.json 正被别人写到一半（解析失败）时什么都不做。
func RepairHooks() error {
	if config.Sandboxed() {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cmd := HookCommand()
	// 先不拿锁看一眼：绝大多数时候 hooks 都在，不用排队等正在跑的同步
	if ok, err := hooksConfigured(cfg.ClaudeDir, cmd); err != nil || ok {
		return err
	}
	log := logging.New(true)
	if err := withSettingsLock(func() error { return ensureHooks(cfg.ClaudeDir, cmd) }); err != nil {
		log.Printf("补回 Claude Code hooks 失败: %v", err)
		return err
	}
	log.Printf("Claude Code hooks 不见了（可能是 CC Switch 等工具改写了 settings.json），已自动补回")
	return nil
}

// RemoveAutoSync 卸载：删掉本工具的 hooks、定时任务和开机自启（别的工具的 hooks 不动）。
// 沙箱模式不碰定时任务（它是全机共用的，删了会把真实环境的也删掉）。
func RemoveAutoSync() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	unhook := withSettingsLock(func() error { return removeHooks(cfg.ClaudeDir) })
	if config.Sandboxed() {
		return unhook
	}
	return errors.Join(unhook, platform.RemoveSchedule(), platform.RemoveAutostart())
}

// withSettingsLock 改写 settings.json 时拿同步锁：同步也会改 settings.json 里的安全字段（语言等），
// 两边同时读—改—写会互相盖掉。只锁写 settings 这一小段，定时任务、开机自启不受影响。
func withSettingsLock(fn func() error) error {
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// syncAutostart 按设置登记或取消开机自启；登记的不是当前 exe 就改过来。
func syncAutostart(cfg *config.Config) error {
	cur, ok := platform.Autostart()
	if cfg.NoAutostart {
		if ok {
			return platform.RemoveAutostart()
		}
		return nil
	}
	if ok && cur == platform.AutostartCommand(ExePath()) {
		return nil
	}
	if err := platform.SetAutostart(ExePath()); err != nil {
		return fmt.Errorf(i18n.T("设置开机自启失败: %w"), err)
	}
	return nil
}

// autoSyncStatus hooks / 定时任务是否已装好且指向当前 exe。
func autoSyncStatus(cfg *config.Config) (hooks, schedule bool) {
	hooks, _ = hooksConfigured(cfg.ClaudeDir, HookCommand())
	return hooks, scheduleCurrent()
}

// scheduleCurrent 定时任务存在、调用的就是当前 exe、间隔也是当前的设定（防止旧版装的任务一直不更新）。
func scheduleCurrent() bool {
	exe, intervalMin, ok := platform.ScheduledInfo()
	return ok && intervalMin == platform.ScheduleIntervalMin && fsutil.SamePath(exe, ExePath())
}

// ---------- ~/.claude/settings.json 里的 hooks ----------

func settingsPath(claudeDir string) string { return filepath.Join(claudeDir, "settings.json") }

// hooksConfigured 每个 hook 事件下都有本工具的命令，且命令和 command 完全一致。
func hooksConfigured(claudeDir, command string) (bool, error) {
	m, err := readClaudeSettings(claudeDir)
	if err != nil {
		return false, err
	}
	hooks, _ := m["hooks"].(map[string]any)
	for _, event := range hookEvents {
		found := false
		forEachHook(hooks, event, func(h map[string]any) {
			if cmd, _ := h["command"].(string); cmd == command {
				found = true
			}
		})
		if !found {
			return false, nil
		}
	}
	return true, nil
}

// ensureHooks 把本工具的 hook 合并进 settings.json：缺的补上，路径不对的改过来，
// 其他配置和别的工具的 hooks 全部保留。没有变化时不写文件。
func ensureHooks(claudeDir, command string) error {
	m, before, err := readClaudeSettingsVersion(claudeDir)
	if err != nil {
		return err
	}
	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		m["hooks"] = hooks
	}
	changed := false
	for _, event := range hookEvents {
		found := false
		forEachHook(hooks, event, func(h map[string]any) {
			if cmd, _ := h["command"].(string); isOurHook(cmd) {
				found = true
				if cmd != command {
					h["command"] = command
					changed = true
				}
			}
		})
		if !found {
			list, _ := hooks[event].([]any)
			hooks[event] = append(list, map[string]any{
				"hooks": []any{map[string]any{"type": "command", "command": command}},
			})
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeClaudeSettings(claudeDir, m, before)
}

// removeHooks 删掉本工具的 hook；删完变空的分组、事件也一并删掉。
func removeHooks(claudeDir string) error {
	m, before, err := readClaudeSettingsVersion(claudeDir)
	if err != nil {
		return err
	}
	hooks, _ := m["hooks"].(map[string]any)
	changed := false
	for event, groups := range hooks {
		list, _ := groups.([]any)
		var keptGroups []any
		for _, g := range list {
			gm, _ := g.(map[string]any)
			entries, _ := gm["hooks"].([]any)
			var kept []any
			for _, e := range entries {
				em, _ := e.(map[string]any)
				if cmd, _ := em["command"].(string); isOurHook(cmd) {
					changed = true
					continue
				}
				kept = append(kept, e)
			}
			if len(kept) > 0 {
				gm["hooks"] = kept
				keptGroups = append(keptGroups, gm)
			}
		}
		if len(keptGroups) > 0 {
			hooks[event] = keptGroups
		} else {
			delete(hooks, event)
		}
	}
	if !changed {
		return nil
	}
	if len(hooks) == 0 {
		delete(m, "hooks")
	}
	return writeClaudeSettings(claudeDir, m, before)
}

// forEachHook 遍历某个事件下所有分组里的每条 hook（可以原地修改）。
func forEachHook(hooks map[string]any, event string, fn func(h map[string]any)) {
	list, _ := hooks[event].([]any)
	for _, g := range list {
		gm, _ := g.(map[string]any)
		entries, _ := gm["hooks"].([]any)
		for _, e := range entries {
			if em, ok := e.(map[string]any); ok {
				fn(em)
			}
		}
	}
}

// readClaudeSettings 读 Claude Code 的 settings.json（不存在当作空配置）。
func readClaudeSettings(claudeDir string) (map[string]any, error) {
	m, _, err := readClaudeSettingsVersion(claudeDir)
	return m, err
}

func readClaudeSettingsVersion(claudeDir string) (map[string]any, []byte, error) {
	data, err := os.ReadFile(settingsPath(claudeDir))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf(i18n.T("settings.json 解析失败 %s: %w"), settingsPath(claudeDir), err)
	}
	return m, data, nil
}

func writeClaudeSettings(claudeDir string, m map[string]any, before []byte) error {
	current, err := os.ReadFile(settingsPath(claudeDir))
	if errors.Is(err, os.ErrNotExist) {
		current = nil
	} else if err != nil {
		return err
	}
	if !bytes.Equal(before, current) {
		return errors.New(i18n.T("settings.json 刚被其他程序修改，本轮先不重写"))
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(settingsPath(claudeDir), data)
}
