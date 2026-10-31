package service

import (
	"sort"
	"strings"

	"github.com/ALFO111111/agent-sync/internal/ccswitch"
	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/state"
)

// Status 本机概况（GUI 首页、向导判断是否已初始化）。
type Status struct {
	Initialized bool            `json:"initialized"`
	DeviceName  string          `json:"deviceName"`
	DeviceID    string          `json:"deviceId"`
	LastSync    string          `json:"lastSync"`   // 已格式化（Wails 绑定不认 time.Time）
	LastSyncAt  int64           `json:"lastSyncAt"` // 毫秒时间戳，界面算"3 分钟前"用；从没同步过为 0
	Projects    int             `json:"projects"`   // 本机已绑定的项目数
	Devices     []DeviceSummary `json:"devices"`    // 其他设备（取上次同步时的缓存，不连云端）
	Stats       state.SyncStats `json:"stats"`      // 上次同步的统计
	Syncing     bool            `json:"syncing"`    // 由 GUI 层填（只有它知道自己有没有在跑）
	// 自动同步（hooks + 定时任务）是否就绪。界面只在没就绪时提示，平时不展示。
	AutoSync       bool         `json:"autoSync"`
	AutoSyncIssue  string       `json:"autoSyncIssue,omitempty"` // 没就绪的原因
	Sandboxed      bool         `json:"sandboxed"`               // 沙箱模式（不装自动同步）
	KeyMismatch    bool         `json:"keyMismatch"`             // 别的设备改了同步口令，本机要输入新口令
	Changes        []ChangeItem `json:"changes"`                 // 上次同步改动了哪些文件
	Conflicts      int          `json:"conflicts"`               // 待人工解决的冲突数
	PendingContent []string     `json:"pendingContent"`          // 需要选择本工具还是 CC Switch 的类别
}

// ChangeItem 本轮明细里的一条（见 state.Change）。
type ChangeItem struct {
	Kind    string `json:"kind"`
	PID     string `json:"pid"`
	Tool    string `json:"tool,omitempty"`
	Project string `json:"project"`
	Rel     string `json:"rel"`
}

// changeItems 本轮明细，项目换成显示名。
func changeItems(st *state.State, changes []state.Change) []ChangeItem {
	out := []ChangeItem{}
	for _, c := range changes {
		it := ChangeItem{Kind: c.Kind, PID: c.PID, Tool: c.Tool, Project: c.Dir, Rel: c.Rel}
		if b := st.Bindings[c.PID]; b != nil && b.Display != "" {
			it.Project = b.Display
		}
		out = append(out, it)
	}
	return out
}

// LoadStatus 未初始化不算错误，返回 Initialized=false。
func LoadStatus() *Status {
	s := &Status{Sandboxed: config.Sandboxed()}
	cfg, err := config.Load()
	if err != nil {
		return s
	}
	s.Initialized = true
	s.DeviceName = cfg.DeviceName
	s.DeviceID = cfg.DeviceID
	s.Devices = []DeviceSummary{}
	s.Changes = []ChangeItem{}
	s.PendingContent = []string{}
	if conflicts, err := engine.ListConflicts(); err == nil {
		s.Conflicts = len(conflicts)
	}
	ccEnabled := ccswitch.DetectAt(cfg.Home) == ccswitch.Enabled
	for _, category := range config.ContentCategories {
		if cfg.ContentEnabled(category) && cfg.ContentPending(category, ccEnabled) {
			s.PendingContent = append(s.PendingContent, category)
		}
	}
	if st, err := state.Load(); err == nil {
		s.LastSync = fmtTime(st.LastSync)
		if !st.LastSync.IsZero() {
			s.LastSyncAt = st.LastSync.UnixMilli()
		}
		s.Stats = st.LastStats
		s.Projects = len(st.Bindings)
		s.KeyMismatch = st.KeyMismatch
		s.Changes = changeItems(st, st.LastChanges)
		for id, d := range st.Devices {
			s.Devices = append(s.Devices, deviceSummary(id, d.Info))
		}
		sort.Slice(s.Devices, func(i, j int) bool { return s.Devices[i].Name < s.Devices[j].Name })
	}
	if s.Sandboxed {
		s.AutoSyncIssue = i18n.T("沙箱模式（设置了 AGENT_SYNC_HOME），不安装自动同步，只能手动同步")
		return s
	}
	hooks, schedule := autoSyncStatus(cfg)
	var issues []string
	if !hooks {
		issues = append(issues, i18n.T("Claude Code hooks 未安装"))
	}
	if !schedule {
		issues = append(issues, i18n.T("定时同步任务未安装"))
	}
	s.AutoSync = len(issues) == 0
	s.AutoSyncIssue = strings.Join(issues, i18n.T("，"))
	return s
}
