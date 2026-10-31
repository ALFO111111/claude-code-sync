package service

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/logging"
	"github.com/ALFO111111/agent-sync/internal/watch"
)

// NewWatchers 要盯的文件和目录（还没 Start）。图形界面常驻时（Windows 托盘）和 macOS 的后台进程 daemon 共用：
//
//	~/.claude/projects 等      内容一有变化就调 onChange（同步）
//	~/.claude/settings.json    被别的工具（如 CC Switch 切换供应商）改写后，检查 hooks 还在不在，不在就补回，再同步
//	~/.codex/sessions 等       Codex 的规则、技能、会话；不递归整个 .codex（里面的 SQLite、日志一直在写，会空转）
func NewWatchers(cfg *config.Config, onChange func()) []*watch.Watcher {
	ws := []*watch.Watcher{
		watch.New(cfg.ProjectsDir(), onChange),
		watch.NewFile(filepath.Join(cfg.ClaudeDir, "settings.json"), func() {
			RepairHooks()
			onChange()
		}),
		watch.NewFile(filepath.Join(cfg.ClaudeDir, "CLAUDE.md"), onChange),
		watch.NewFile(cfg.ClaudeStatePath(), onChange),
		watch.NewFile(filepath.Join(cfg.CodexDir, "AGENTS.md"), onChange),
		watch.NewFile(filepath.Join(cfg.CodexDir, "AGENTS.override.md"), onChange),
		watch.New(filepath.Join(cfg.Home, ".agents", "skills"), onChange),
		watch.New(filepath.Join(cfg.CodexDir, "sessions"), onChange),
	}
	// 排除某些子目录：Claude skills 下的 synced/、.trash/ 归 CC Switch；Codex skills 下的 .system 是它自带的
	excluding := func(root string, skip func(first string, parts []string) bool) *watch.Watcher {
		w := watch.New(root, onChange)
		w.SetFilter(func(path string) bool {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return false
			}
			parts := strings.Split(strings.ToLower(filepath.ToSlash(rel)), "/")
			return !skip(parts[0], parts)
		})
		return w
	}
	ws = append(ws, excluding(filepath.Join(cfg.CodexDir, "skills"), func(first string, _ []string) bool { return first == ".system" }))
	for _, dir := range []string{"plans", "skills", "agents", "commands", "tasks"} {
		root := filepath.Join(cfg.ClaudeDir, dir)
		if dir != "skills" {
			ws = append(ws, watch.New(root, onChange))
			continue
		}
		ws = append(ws, excluding(root, func(_ string, parts []string) bool {
			for _, p := range parts {
				if p == "synced" || p == ".trash" {
					return true
				}
			}
			return false
		}))
	}
	return ws
}

// RunDaemon 没有窗口的后台同步（macOS 登录时由 launchd 启动 `agent-sync daemon`）：
// 先全量同步一次，之后文件一变就同步；ctx 结束时退出。还没初始化时每分钟看一眼，初始化后自动开始。
func RunDaemon(ctx context.Context) error {
	var cfg *config.Config
	for {
		var err error
		if cfg, err = config.Load(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
		}
	}
	kick := make(chan struct{}, 1)
	trigger := func() {
		select {
		case kick <- struct{}{}:
		default: // 已经有一次在排队了，合并成一次
		}
	}
	ws := NewWatchers(cfg, trigger)
	for _, w := range ws {
		if err := w.Start(); err != nil {
			logging.New(true).Printf("文件监听启动失败: %v", err)
		}
	}
	defer func() {
		for _, w := range ws {
			w.Stop()
		}
	}()
	logging.New(true).Printf("后台同步已启动")
	trigger()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-kick:
			// 失败照常写进 sync.log；下一次文件变化或每小时的定时任务会再试
			Sync(ctx, SyncRequest{Quiet: true, WaitLock: 30 * time.Second})
		}
	}
}
