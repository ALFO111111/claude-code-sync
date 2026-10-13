package gui

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/service"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
	"github.com/ALFO111111/claude-code-sync/internal/watch"
)

// App 是暴露给前端（Vue）的绑定对象，所有导出方法都能在 JS 里直接调（frontend/wailsjs/go/gui/App 自动生成）。
// 这一层只做薄包装：业务都在 service 包。
//
// 前端能用的全部接口：
//
//	状态     GetStatus
//	同步     SyncNow                                 + 事件 "sync"（SyncEvent）
//	项目     ListProjects  ListBound  ClaimProject  IgnoreProject  UnignoreProject  PickDirectory
//	会话     ListSessions  ReadSession
//	对话框   Confirm（危险操作前的系统确认框）
//	设置     GetSettings  SaveSettings  TestStorage
//	语言     GetLanguage  SetLanguage（立即生效：界面、托盘、错误信息、日志）
//	自动同步 RepairAutoSync（平时自动装好，不展示；GetStatus.autoSync=false 时界面才提示修复）
//	日志     ReadLog  ClearLog
//	向导     DefaultDeviceName  Initialize
type App struct {
	ctx       context.Context // Wails 运行时上下文（发事件、弹对话框用）
	syncing   atomic.Bool
	quitting  atomic.Bool      // 托盘点了"退出"：OnBeforeClose 看到这个标记才真关窗口
	ensuring  sync.WaitGroup   // 启动时的自动同步检查；GetStatus 等它跑完，免得首页闪一下"未安装"
	syncMu    sync.Mutex       // 串行化同步：用户操作（认领/忽略）排队等上一轮，后台同步忙时记一笔、跑完补一轮
	rerun     atomic.Bool      // 同步进行中又来了后台触发
	watchers  []*watch.Watcher // 文件监听（程序开着才生效）：会话有变化就同步；settings.json 变了就检查 hooks
	watcherMu sync.Mutex
	tray      *tray // 系统托盘（测试里为 nil）
}

func NewApp() *App { return &App{} }

// startup 窗口启动时调用：后台确保自动同步（hooks + 定时任务）已装好且指向当前 exe，
// 然后启动文件监听（会话一有变化就自动同步）。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.ensuring.Add(1)
	go func() {
		defer a.ensuring.Done()
		if service.LoadStatus().Initialized {
			service.EnsureAutoSync() // 失败写日志，首页会提示
			a.startWatcher()
		}
	}()
}

// startWatcher 启动文件监听（已启动则忽略）。初始化完成前、目录不存在时静默跳过。
//
//	~/.claude/projects       会话一有变化就自动同步
//	~/.claude/settings.json  被别的工具（如 CC Switch 切换供应商）改写后，检查 hooks 还在不在，不在就补回
func (a *App) startWatcher() {
	a.watcherMu.Lock()
	defer a.watcherMu.Unlock()
	if a.watchers != nil {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		return
	}
	for _, w := range []*watch.Watcher{
		watch.New(cfg.ProjectsDir(), func() { go a.runSync(service.SyncRequest{}, false) }),
		watch.NewFile(filepath.Join(cfg.ClaudeDir, "settings.json"), func() { service.RepairHooks() }),
	} {
		if w.Start() == nil {
			a.watchers = append(a.watchers, w)
		}
	}
}

// shutdown 程序真正退出时调用：停掉文件监听。
func (a *App) shutdown(context.Context) {
	a.watcherMu.Lock()
	defer a.watcherMu.Unlock()
	for _, w := range a.watchers {
		w.Stop()
	}
	a.watchers = nil
}

func (a *App) emit(event string, data any) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, event, data)
	}
}

// ---------- 状态 ----------

func (a *App) GetStatus() *service.Status {
	a.ensuring.Wait()
	s := service.LoadStatus()
	s.Syncing = a.syncing.Load()
	return s
}

// ---------- 同步 ----------

// SyncEvent 事件 "sync" 的内容：开始时 Running=true；结束时 Running=false，带统计和错误。
type SyncEvent struct {
	Running bool            `json:"running"`
	Stats   state.SyncStats `json:"stats"`
	Error   string          `json:"error,omitempty"`
}

var errBusy = i18n.Err("正在同步中，请等这一轮结束再试")

// runSync 界面发起的所有同步（立即同步、认领、忽略、文件监听）都走这里，用 syncMu 串行：
// wait=true（用户主动点的认领/忽略）排队等上一轮结束再跑，前端不用禁用按钮；
// wait=false（立即同步/文件监听这类后台触发）发现正在跑不排队，记一笔，等这轮跑完补一轮全量。
// 开始/结束都推 "sync" 事件，界面统一显示"同步中"。
func (a *App) runSync(req service.SyncRequest, wait bool) error {
	if wait {
		a.syncMu.Lock()
	} else if !a.syncMu.TryLock() {
		a.rerun.Store(true) // 记一笔：这轮跑完再补一轮，免得这次的变化要等下一个触发
		return errBusy
	}
	defer a.syncMu.Unlock()
	err := a.syncOnce(req)
	// 跑的过程中又来了后台触发：还在锁里补跑一轮全量（补跑没有新变化就不写文件，不会无限循环）
	for a.rerun.Swap(false) {
		a.syncOnce(service.SyncRequest{})
	}
	return err
}

// syncOnce 跑一轮同步，开始/结束推 "sync" 事件、更新托盘。调用方持有 syncMu。
func (a *App) syncOnce(req service.SyncRequest) error {
	a.syncing.Store(true)
	a.emit("sync", SyncEvent{Running: true})
	a.tray.syncStarted()
	req.Quiet = true
	req.WaitLock = 30 * time.Second
	stats, err := service.Sync(context.Background(), req)
	ev := SyncEvent{Stats: stats}
	if err != nil {
		ev.Error = err.Error()
	}
	a.syncing.Store(false) // 先放开再发事件：前端收到事件后查 GetStatus 应看到 syncing=false
	a.emit("sync", ev)
	a.tray.syncFinished(ev.Error)
	return err
}

// SyncNow 后台全量同步，立即返回；结果看 "sync" 事件。已在同步时忽略。
func (a *App) SyncNow() {
	go a.runSync(service.SyncRequest{}, false)
}

// ---------- 项目 ----------

func (a *App) ListProjects() (*service.ProjectsOverview, error) {
	return service.LoadProjects(context.Background())
}

// ClaimProject 认领其他设备的项目到本机目录（顺带跑一轮同步，等它跑完才返回；
// 若上一轮同步还在跑，排队等它结束，不会报忙）。
func (a *App) ClaimProject(pid, root string) error {
	if root == "" {
		return errors.New(i18n.T("请先选择本机目录"))
	}
	return a.runSync(service.SyncRequest{ClaimPID: pid, ClaimRoot: root}, true)
}

// IgnoreProject 忽略一个项目：待认领的不再提示；已绑定的停止同步（本机、云端文件都不删）。
func (a *App) IgnoreProject(pid string) error {
	return a.runSync(service.SyncRequest{IgnorePID: pid}, true)
}

// UnignoreProject 取消忽略，顺带同步一轮：本机有会话的自动绑回来，其他设备的重新出现在待认领里。
func (a *App) UnignoreProject(pid string) error {
	return a.runSync(service.SyncRequest{UnignorePID: pid}, true)
}

// ListBound 本机已绑定的项目（只读本地，不连云端，会话页用）。
func (a *App) ListBound() ([]service.BoundProject, error) { return service.ListBound() }

// Confirm 弹系统确认框（危险操作前用），点"是"返回 true。
func (a *App) Confirm(title, message string) (bool, error) {
	if a.ctx == nil {
		return false, errors.New(i18n.T("界面还没就绪"))
	}
	r, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
		Type: wailsRuntime.QuestionDialog, Title: title, Message: message,
	})
	return r == "Yes", err
}

// PickDirectory 弹系统目录选择框（认领项目时用）。用户取消返回空字符串。
func (a *App) PickDirectory() (string, error) {
	if a.ctx == nil {
		return "", errors.New(i18n.T("界面还没就绪"))
	}
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{Title: i18n.T("选择本项目在本机的目录")})
}

// ---------- 会话 ----------

func (a *App) ListSessions(pid string) ([]service.SessionInfo, error) {
	return service.ListSessions(pid)
}

func (a *App) ReadSession(pid, rel string) ([]service.SessionMessage, error) {
	return service.ReadSession(pid, rel)
}

// ---------- 设置 ----------

func (a *App) GetSettings() (*service.Settings, error) { return service.LoadSettings() }

// SaveSettings 一次保存全部设置；存储配置变了会先测连接，连不上不保存。
// Storage.AccessKeySecret 留空 = 沿用已保存的。
func (a *App) SaveSettings(s service.Settings) error {
	return service.SaveSettings(context.Background(), s)
}

// TestStorage 向导/设置页的"测试连接"。Secret 留空时用已保存的。
func (a *App) TestStorage(sc storage.Config) error {
	return service.TestStorage(context.Background(), service.WithSavedSecret(sc))
}

// ---------- 语言 ----------

func (a *App) GetLanguage() service.LanguageInfo { return service.GetLanguage() }

// SetLanguage 切换语言（空 = 跟随系统），返回切换后的实际语言。托盘菜单同时刷新。
func (a *App) SetLanguage(lang string) (service.LanguageInfo, error) {
	err := service.SetLanguage(lang)
	a.tray.languageChanged()
	return service.GetLanguage(), err
}

// ---------- 自动同步（hooks / 定时任务）----------

// RepairAutoSync 重新安装自动同步。平时启动时已自动处理，只在首页提示"没装好"时给用户点。
func (a *App) RepairAutoSync() error { return service.EnsureAutoSync() }

// ---------- 日志 ----------

func (a *App) ReadLog() (*service.LogView, error) { return service.ReadLog(256 << 10) }

func (a *App) ClearLog() error { return service.ClearLog() }

// ---------- 向导 ----------

// DefaultDeviceName 本机登录用户名；取不到返回空串（界面只显示灰色示例，不预填）。
func (a *App) DefaultDeviceName() string { return service.DefaultDeviceName() }

// Initialize 向导最后一步：执行完整初始化，并自动装好 hooks 和定时任务。
// 返回 "created"（云端首次写入口令校验）或 "matched"。自动同步装失败不算初始化失败（首页会提示修复）。
func (a *App) Initialize(p service.InitParams) (string, error) {
	res, err := service.Initialize(context.Background(), p)
	if err != nil {
		return "", err
	}
	service.EnsureAutoSync()
	a.startWatcher()
	return res.Keycheck, nil
}
