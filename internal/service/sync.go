package service

import (
	"context"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/engine"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// SyncRequest 一次同步的参数。Claim/Ignore 非空时顺带认领/忽略一个其他设备的项目。
type SyncRequest struct {
	Quiet       bool          // 日志只写文件，不打终端
	OnlyCwd     string        // 只同步这个工作目录对应的项目
	Dry         bool          // 只看不动
	WaitLock    time.Duration // 别的同步正在跑时最多等多久
	ClaimPID    string
	ClaimRoot   string
	IgnorePID   string
	UnignorePID string
}

// Sync 跑一轮同步，返回统计（出错时统计也是有效的，记录了出错前做了什么）。
func Sync(ctx context.Context, req SyncRequest) (state.SyncStats, error) {
	e, err := engine.New(req.Quiet)
	if err != nil {
		return state.SyncStats{}, err
	}
	opts := engine.Options{OnlyCwd: req.OnlyCwd, Dry: req.Dry, WaitLock: req.WaitLock, Ignore: req.IgnorePID, Unignore: req.UnignorePID}
	if req.ClaimPID != "" {
		opts.Claim = &engine.Claim{PID: req.ClaimPID, Root: req.ClaimRoot}
	}
	err = e.Sync(ctx, opts)
	return e.Stats(), err
}

// HookSync 被 Claude Code hook 调用：只同步 cwd 对应的项目。
// 拉到了其他设备的新内容时返回一句路径对照提示（hook 的 stdout 会成为 AI 的上下文），否则返回空串。
// 出错一律静默（日志在 sync.log），hook 里不打扰用户。
func HookSync(ctx context.Context, cwd string) string {
	e, err := engine.New(true)
	if err != nil {
		return ""
	}
	if err := e.Sync(ctx, engine.Options{OnlyCwd: cwd, WaitLock: 5 * time.Second}); err != nil {
		return ""
	}
	if e.Stats().Pulled == 0 {
		return ""
	}
	b := e.BindCwd(cwd)
	if b == nil {
		return ""
	}
	foreign := e.ForeignRoots(b.PID)
	if len(foreign) == 0 {
		return ""
	}
	return i18n.T("[claude-code-sync] 此项目的会话已从其他设备同步。记录中的路径 %s 即本机的 %s，请按本机路径理解。",
		strings.Join(foreign, i18n.T("、")), b.Root)
}
