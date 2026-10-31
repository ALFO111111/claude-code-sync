package service

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/platform"
	"github.com/ALFO111111/agent-sync/internal/state"
)

// 工具清单：界面顶部的工具切换、首页的工具卡片、会话页和"同步内容"页都按它生成。
// 以后接入新工具（pi、hermes……）：在 toolDefs 里加一项，实现它的 sessionSource（能浏览会话的话），
// 再在 config 里登记它的同步内容类别。界面不用为某个工具单独写代码。

// ToolInfo 界面看到的一个工具。
type ToolInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Installed    bool   `json:"installed"`    // 本机有它的数据目录
	Dir          string `json:"dir"`          // 数据目录（~/.claude、~/.codex……）
	Sessions     bool   `json:"sessions"`     // 支持浏览会话
	SessionCount int    `json:"sessionCount"` // 本机的会话数
	ContentOn    int    `json:"contentOn"`    // 打开了几类同步内容
	ContentTotal int    `json:"contentTotal"`
	ResumeCmd    string `json:"resumeCmd"` // 继续会话用的命令名（claude / codex），界面按钮文字用
}

// sessionSource 一个工具的会话从哪来、怎么读、怎么继续。
type sessionSource interface {
	// files 本机所有会话文件，按项目归好。
	files(cfg *config.Config, st *state.State) ([]sessionFile, error)
	// info 一个会话在列表里显示的内容（读摘要有缓存）。
	info(cfg *config.Config, st *state.State, f sessionFile) (SessionInfo, bool)
	// parse 把会话文件解析成消息（查看器、搜索用）。
	parse(data []byte) []SessionMessage
	// resume 在终端里继续这个会话：在哪个目录、跑什么命令。
	resume(cfg *config.Config, f sessionFile) (dir string, argv []string, err error)
}

// sessionFile 一个会话文件：属于哪个绑定项目、相对路径、绝对路径。
type sessionFile struct {
	b   *state.Binding
	rel string
	abs string
}

type toolDef struct {
	id, name   string
	resumeCmd  string
	dir        func(cfg *config.Config) string
	categories []string
	sessions   sessionSource
}

var toolDefs = []toolDef{
	{
		id: config.ToolClaude, name: "Claude Code", resumeCmd: "claude",
		dir:        func(cfg *config.Config) string { return cfg.ClaudeDir },
		categories: config.ContentCategories,
		sessions:   claudeSessions{},
	},
	{
		id: config.ToolCodex, name: "Codex", resumeCmd: "codex",
		dir:        func(cfg *config.Config) string { return cfg.CodexDir },
		categories: config.CodexContentCategories,
		sessions:   codexSessions{},
	},
}

func toolByID(id string) (*toolDef, error) {
	if id == "" {
		id = config.ToolClaude
	}
	for i := range toolDefs {
		if toolDefs[i].id == id {
			return &toolDefs[i], nil
		}
	}
	return nil, errors.New(i18n.T("不认识的工具: %s", id))
}

// ListTools 所有支持的工具和它们在本机的概况。
func ListTools() ([]ToolInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	out := []ToolInfo{}
	for _, d := range toolDefs {
		ti := ToolInfo{ID: d.id, Name: d.name, Dir: d.dir(cfg), Sessions: d.sessions != nil, ContentTotal: len(d.categories), ResumeCmd: d.resumeCmd}
		if fi, err := os.Stat(ti.Dir); err == nil && fi.IsDir() {
			ti.Installed = true
		}
		for _, c := range d.categories {
			if cfg.ContentEnabledFor(d.id, c) {
				ti.ContentOn++
			}
		}
		if d.sessions != nil {
			files, _ := d.sessions.files(cfg, st)
			ti.SessionCount = len(files)
		}
		out = append(out, ti)
	}
	return out, nil
}

// ToolProject 会话页左边的项目：本机已绑定、而且这个工具在里面有会话。
type ToolProject struct {
	BoundProject
	Sessions int `json:"sessions"`
}

// ListToolProjects 这个工具有会话的项目（会话多的项目不一定排前面，按名字排，位置稳定好找）。
func ListToolProjects(tool string) ([]ToolProject, error) {
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return nil, err
	}
	files, err := d.sessions.files(cfg, st)
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, f := range files {
		count[f.b.PID]++
	}
	out := []ToolProject{}
	for _, b := range boundOf(st) {
		if count[b.PID] > 0 {
			out = append(out, ToolProject{BoundProject: b, Sessions: count[b.PID]})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Display) < strings.ToLower(out[j].Display)
	})
	return out, nil
}

func openTool(tool string) (*toolDef, *config.Config, *state.State, error) {
	d, err := toolByID(tool)
	if err != nil {
		return nil, nil, nil, err
	}
	if d.sessions == nil {
		return nil, nil, nil, errors.New(i18n.T("这个工具还不支持浏览会话"))
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	return d, cfg, st, nil
}

// ListToolSessions 一个项目里这个工具的会话，新的排前面。
func ListToolSessions(tool, pid string) ([]SessionInfo, error) {
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return nil, err
	}
	files, err := d.sessions.files(cfg, st)
	if err != nil {
		return nil, err
	}
	out := []SessionInfo{}
	for _, f := range files {
		if f.b.PID != pid {
			continue
		}
		if s, ok := d.sessions.info(cfg, st, f); ok {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mtime > out[j].Mtime })
	return out, nil
}

// findSession 按项目 + 相对路径找到会话文件（只认 files 列出来的，不会读到别处）。
func findSession(d *toolDef, cfg *config.Config, st *state.State, pid, rel string) (sessionFile, error) {
	files, err := d.sessions.files(cfg, st)
	if err != nil {
		return sessionFile{}, err
	}
	for _, f := range files {
		if f.b.PID == pid && f.rel == rel {
			return f, nil
		}
	}
	return sessionFile{}, errors.New(i18n.T("找不到这个会话（可能已被删除）"))
}

// ReadToolSession 读一个会话的全部消息（查看器）。
func ReadToolSession(tool, pid, rel string) ([]SessionMessage, error) {
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return nil, err
	}
	f, err := findSession(d, cfg, st, pid, rel)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(f.abs)
	if err != nil {
		return nil, err
	}
	return d.sessions.parse(data), nil
}

// SearchToolSessions 在这个工具的所有会话里搜消息内容（规则同 SearchSessions）。
func SearchToolSessions(tool, q string) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	out := []SearchHit{}
	if len([]rune(q)) < 2 {
		return out, nil
	}
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return nil, err
	}
	files, err := d.sessions.files(cfg, st)
	if err != nil {
		return nil, err
	}
	lq := strings.ToLower(q)
	type cand struct {
		hit   SearchHit
		mtime time.Time
	}
	var cands []cand
	for _, f := range files {
		data, err := os.ReadFile(f.abs)
		if err != nil || !strings.Contains(strings.ToLower(string(data)), lq) {
			continue
		}
		h := SearchHit{PID: f.b.PID, Project: f.b.Display, Rel: f.rel}
		for _, m := range d.sessions.parse(data) {
			if i := strings.Index(strings.ToLower(m.Text), lq); i >= 0 {
				if h.Count++; h.Snippet == "" {
					h.Snippet = snippet(m.Text, i, lq)
				}
			}
		}
		if h.Count == 0 {
			continue
		}
		c := cand{hit: h}
		if s, ok := d.sessions.info(cfg, st, f); ok {
			c.hit.Mtime = s.Mtime
			if c.hit.Title = s.Title; s.Title == "" {
				c.hit.Title = s.Preview
			}
		}
		if fi, err := os.Stat(f.abs); err == nil {
			c.mtime = fi.ModTime()
		}
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mtime.After(cands[j].mtime) })
	for i, c := range cands {
		if i == maxSearchHits {
			break
		}
		out = append(out, c.hit)
	}
	return out, nil
}

// ToolResumeCommand 继续一个会话的命令（可以直接粘贴到终端）。
func ToolResumeCommand(tool, pid, rel string) (*ResumeInfo, error) {
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return nil, err
	}
	f, err := findSession(d, cfg, st, pid, rel)
	if err != nil {
		return nil, err
	}
	dir, argv, err := d.sessions.resume(cfg, f)
	if err != nil {
		return nil, err
	}
	return &ResumeInfo{Dir: dir, Command: `cd "` + dir + `"; ` + strings.Join(argv, " ")}, nil
}

// OpenToolSession 开一个终端窗口，在项目目录里继续这个会话。
func OpenToolSession(tool, pid, rel string) error {
	d, cfg, st, err := openTool(tool)
	if err != nil {
		return err
	}
	f, err := findSession(d, cfg, st, pid, rel)
	if err != nil {
		return err
	}
	dir, argv, err := d.sessions.resume(cfg, f)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return errors.New(i18n.T("项目目录不存在: %s", dir))
	}
	return platform.OpenTerminal(dir, argv...)
}

// ---------- Claude Code ----------

// claudeSessions Claude 的会话就在每个绑定项目的 ~/.claude/projects/<目录> 下（顶层 .jsonl）。
type claudeSessions struct{}

func (claudeSessions) files(cfg *config.Config, st *state.State) ([]sessionFile, error) {
	var out []sessionFile
	for _, b := range st.Bindings {
		dir := filepath.Join(cfg.ProjectsDir(), b.Dir)
		entries, _ := os.ReadDir(dir)
		for _, ent := range entries {
			if !ent.IsDir() && strings.HasSuffix(ent.Name(), ".jsonl") {
				out = append(out, sessionFile{b: b, rel: ent.Name(), abs: filepath.Join(dir, ent.Name())})
			}
		}
	}
	return out, nil
}

func (claudeSessions) info(cfg *config.Config, st *state.State, f sessionFile) (SessionInfo, bool) {
	fi, err := os.Stat(f.abs)
	if err != nil {
		return SessionInfo{}, false
	}
	s := SessionInfo{Name: f.rel, Rel: f.rel, Size: fi.Size(), Mtime: fmtTime(fi.ModTime())}
	var ok bool
	if s.Lines, s.Preview, s.Title, ok = cachedSummary(f.abs, fi); !ok {
		return SessionInfo{}, false
	}
	if fs := st.Files[engine.FileKey(f.b.PID, f.rel)]; fs != nil && !fs.Deleted {
		s.Synced = true
	}
	id := strings.TrimSuffix(f.rel, ".jsonl")
	s.ForkOf, s.Resume = st.Forks[id], isSessionID(id)
	return s, true
}

func (claudeSessions) parse(data []byte) []SessionMessage { return parseSession(data) }

func (claudeSessions) resume(cfg *config.Config, f sessionFile) (string, []string, error) {
	id := strings.TrimSuffix(f.rel, ".jsonl")
	if !isSessionID(id) {
		return "", nil, errors.New(i18n.T("只有顶层会话能继续"))
	}
	return f.b.Root, []string{"claude", "--resume", id}, nil
}

// within p 是不是 root 本身或在它下面（不分大小写，\ 和 / 一样）。
func within(root, p string) bool {
	r := strings.TrimRight(strings.ToLower(filepath.ToSlash(fsutil.CleanPath(root))), "/")
	q := strings.ToLower(filepath.ToSlash(fsutil.CleanPath(p)))
	return r != "" && (q == r || strings.HasPrefix(q, r+"/"))
}
