// Package engine 同步引擎：本机 ~/.claude/projects 和云端之间的双向同步。
//
//	engine.go     Engine、Sync 主流程
//	devices.go    设备信息交换（devices/<ID>.json）
//	binding.go    项目发现、自动绑定、认领
//	identity.go   本机目录 → 项目身份证
//	transfer.go   文件逐个比对 → 上传/下载/合并/分叉
//	merge.go      jsonl 按 uuid 合并、分叉、只追加写入
//	backup.go     被替换内容的备份、恢复
//	history.go    输入历史 ~/.claude/history.jsonl（全机一份，只追加合并）
//	keys.go       口令校验、修改口令（全部云端文件换新密钥重新加密）
//	usage.go      云端用量统计
//	translate.go  拉到本机时翻译路径
package engine

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/logging"
	"github.com/ALFO111111/claude-code-sync/internal/project"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// Engine 一次同步会话的全部上下文（配置、状态、云端连接、密钥）。
type Engine struct {
	cfg     *config.Config
	st      *state.State
	store   storage.Storage
	key     []byte
	prevKey []byte // 改口令前的旧密钥（可能为空）：别的设备还没换口令时传上来的文件，用它也能解开
	log     *logging.Logger
	bk      *backups

	devices map[string]state.DeviceInfo // 其他设备（不含本机）
	dry     bool

	// walkFn 遍历项目目录用的函数，默认 filepath.WalkDir；测试可替换以模拟遍历失败。
	walkFn func(root string, fn fs.WalkDirFunc) error

	mu          sync.Mutex
	translators map[string]*translator
	stats       state.SyncStats
	changes     map[string]state.Change // 云端 key → 本轮的变化（同一个文件只留最重要的那种）
	progress    func(done, total int)
	done, total int
}

// New 读配置、密钥、状态并连上云端。quiet=true 时日志只写文件。
func New(quiet bool) (*Engine, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	key, err := config.LoadKey()
	if err != nil {
		return nil, err
	}
	store, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	log := logging.New(quiet)
	return &Engine{
		cfg: cfg, st: st, store: store, key: key, prevKey: config.LoadPrevKey(),
		log:    log,
		bk:     newBackups(log),
		walkFn: filepath.WalkDir,
	}, nil
}

func (e *Engine) Config() *config.Config { return e.cfg }
func (e *Engine) State() *state.State    { return e.st }

// Stats 最近一次 Sync 的统计。
func (e *Engine) Stats() state.SyncStats { return e.stats }

// Changes 最近一次 Sync 改动了哪些文件（最多 state.MaxChanges 条）。
func (e *Engine) Changes() []state.Change { return e.st.LastChanges }

// Devices 其他设备的信息（不含本机），FetchDevices 之后才有。
func (e *Engine) Devices() map[string]state.DeviceInfo { return e.devices }

// FileKey 项目文件在云端的 key：files/<pid 哈希>/<相对路径>。
func FileKey(pid, rel string) string { return "files/" + project.Hash(pid) + "/" + rel }

type Options struct {
	OnlyCwd  string        // 只同步这个工作目录对应的项目（SessionStart hook 用）
	Dry      bool          // 只看不动
	WaitLock time.Duration // 锁被占用时最多等多久
	Claim    *Claim        // 认领一个其他设备的项目
	Ignore   string        // 忽略一个项目：待认领的不再提示；已绑定的停止同步（本机、云端文件都不动）
	Unignore string        // 取消忽略：之后照常自动发现、自动对上，或重新出现在待认领里
	// Progress 文件比对的进度（done 个已处理 / 共 total 个）。在同步用的 goroutine 里调用，要快。
	Progress func(done, total int)
}

// Claim 把其他设备的项目 PID 绑定到本机目录 Root。
type Claim struct{ PID, Root string }

func (e *Engine) Sync(ctx context.Context, opts Options) error {
	unlock, err := state.WaitLock(opts.WaitLock)
	if err != nil {
		return err
	}
	defer unlock()

	// 拿到锁后重新读状态，别的进程可能刚改过
	if e.st, err = state.Load(); err != nil {
		return err
	}
	e.dry = opts.Dry
	e.translators = map[string]*translator{}
	e.stats = state.SyncStats{}
	e.changes = map[string]state.Change{}
	e.progress, e.done, e.total = opts.Progress, 0, 0
	if !e.dry {
		pruneBackups()
	}
	start := time.Now()

	// 先确认口令和云端一致：别的设备改了口令后，本机用旧密钥接着传，会把新旧密钥的文件混在一起
	if err := e.verifyKey(ctx); err != nil {
		if !e.dry {
			state.Save(e.st)
		}
		return err
	}
	if err := e.FetchDevices(ctx); err != nil {
		return fmt.Errorf(i18n.T("读取云端设备信息失败: %w"), err)
	}

	var only *state.Binding
	if opts.OnlyCwd != "" {
		if only = e.BindCwd(opts.OnlyCwd); only == nil {
			if !e.dry {
				return state.Save(e.st)
			}
			return nil
		}
	} else {
		if opts.Unignore != "" {
			delete(e.st.Ignored, opts.Unignore)
			e.log.Printf("已取消忽略项目 %s", opts.Unignore)
		}
		e.discover()
		if opts.Ignore != "" {
			e.ignore(opts.Ignore)
		}
		if opts.Claim != nil {
			if e.bind(opts.Claim.PID, opts.Claim.Root, filepath.Base(opts.Claim.Root), true) != nil {
				delete(e.st.Ignored, opts.Claim.PID)
				e.log.Printf("已认领 %s → %s", opts.Claim.PID, opts.Claim.Root)
			}
		}
		e.autoBind()
	}

	if !e.dry {
		if err := e.publishDevice(ctx); err != nil {
			e.log.Printf("上传设备信息失败: %v", err)
		}
	}
	err = e.syncFiles(ctx, only)
	if herr := e.syncHistory(ctx); herr != nil {
		e.log.Printf("输入历史同步失败: %v", herr)
		e.count(&e.stats.Errors)
	}
	if !e.dry {
		e.st.LastSync = time.Now()
		e.st.LastStats = e.stats
		e.st.LastChanges = e.sortedChanges()
		if serr := state.Save(e.st); serr != nil && err == nil {
			err = serr
		}
	}
	s := e.stats
	if e.dry {
		e.log.Printf("预览完成（未做任何改动）")
	} else if s.Pushed+s.Pulled+s.Merged+s.Forked+s.Errors+s.History > 0 || !e.log.Quiet() {
		e.log.Printf("同步完成：上传 %d，下载 %d，合并 %d，分叉 %d，失败 %d，输入历史补进 %d 条，用时 %.1fs",
			s.Pushed, s.Pulled, s.Merged, s.Forked, s.Errors, s.History, time.Since(start).Seconds())
	}
	if err == nil && s.Errors > 0 {
		err = fmt.Errorf(i18n.T("%d 个文件同步失败，详见日志 %s"), s.Errors, logging.Path())
	}
	return err
}
