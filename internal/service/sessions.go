package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/engine"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// 会话浏览：会话列表 + 完整对话查看器的解析（GUI 会话页用）。只读本地文件，不连云端。

// SessionInfo 会话列表里的一行。
type SessionInfo struct {
	Name    string `json:"name"` // 文件名（<uuid>.jsonl）
	Rel     string `json:"rel"`  // 相对项目目录的路径
	Size    int64  `json:"size"`
	Lines   int    `json:"lines"`   // 记录条数
	Mtime   string `json:"mtime"`   // 已格式化（Wails 绑定不认 time.Time）
	Preview string `json:"preview"` // 首条用户消息摘要
	Synced  bool   `json:"synced"`  // 已同步到云端
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// ListSessions 列出一个绑定项目的顶层会话（子目录里的子代理记录不算），新的排前面。
func ListSessions(pid string) ([]SessionInfo, error) {
	cfg, st, b, err := openBinding(pid)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cfg.ProjectsDir(), b.Dir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []SessionInfo{}, nil // 刚绑定、本机还没聊过也没拉到东西
	}
	if err != nil {
		return nil, err
	}
	out := []SessionInfo{}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".jsonl") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		s := SessionInfo{Name: ent.Name(), Rel: ent.Name(), Size: info.Size(), Mtime: fmtTime(info.ModTime())}
		var ok bool
		if s.Lines, s.Preview, ok = cachedSummary(filepath.Join(dir, ent.Name()), info); !ok {
			continue
		}
		if fs := st.Files[engine.FileKey(pid, s.Rel)]; fs != nil && !fs.Deleted {
			s.Synced = true
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mtime > out[j].Mtime })
	return out, nil
}

// summaryCache 会话列表的摘要缓存：路径 → 上次读到时的大小、修改时间和摘要。
// 每次打开列表都把所有会话整个读一遍太慢（会话文件动辄几 MB），没变的直接用缓存。
var summaryCache sync.Map

type summaryEntry struct {
	size    int64
	mtime   time.Time
	lines   int
	preview string
}

func cachedSummary(path string, info fs.FileInfo) (lines int, preview string, ok bool) {
	if v, hit := summaryCache.Load(path); hit {
		if c := v.(summaryEntry); c.size == info.Size() && c.mtime.Equal(info.ModTime()) {
			return c.lines, c.preview, true
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", false
	}
	lines, preview = sessionSummary(data)
	summaryCache.Store(path, summaryEntry{info.Size(), info.ModTime(), lines, preview})
	return lines, preview, true
}

// sessionSummary 数记录条数，并找首条用户消息当摘要。
func sessionSummary(data []byte) (lines int, preview string) {
	for _, l := range engine.SplitLines(data) {
		lines++
		if preview == "" {
			if m, ok := parseMessageLine(l); ok && m.Role == "user" && m.Text != "" {
				preview = truncate(m.Text, 100)
			}
		}
	}
	return lines, preview
}

// SessionMessage 查看器里的一条消息。
type SessionMessage struct {
	Role  string     `json:"role"` // "user" | "assistant"
	Time  string     `json:"time"`
	Text  string     `json:"text"`
	Tools []ToolCall `json:"tools,omitempty"`
}

// ToolCall 一次工具调用的摘要（默认折叠，点开看）。
type ToolCall struct {
	Name  string `json:"name"`
	Brief string `json:"brief"`
}

// parseSession 把会话 jsonl 解析成消息序列。
// 跳过：工具结果、纯思考、summary、解析失败的行——查看器只看"人说了什么、AI 回了什么、用了什么工具"。
func parseSession(data []byte) []SessionMessage {
	out := []SessionMessage{} // 空会话也返回 []：JSON 里是 null 的话前端会当成"没打开"
	for _, l := range engine.SplitLines(data) {
		m, ok := parseMessageLine(l)
		if ok && (m.Text != "" || len(m.Tools) > 0) {
			out = append(out, m)
		}
	}
	return out
}

func parseMessageLine(line []byte) (SessionMessage, bool) {
	var v struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &v); err != nil {
		return SessionMessage{}, false
	}
	if v.Type != "user" && v.Type != "assistant" {
		return SessionMessage{}, false
	}
	m := SessionMessage{Role: v.Type}
	if t, err := time.Parse(time.RFC3339Nano, v.Timestamp); err == nil {
		m.Time = t.Local().Format("01-02 15:04:05")
	}

	// content 可能是纯字符串（用户消息）或数组
	var text string
	if json.Unmarshal(v.Message.Content, &text) == nil {
		m.Text = text
		return m, true
	}
	var parts []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	if json.Unmarshal(v.Message.Content, &parts) != nil {
		return SessionMessage{}, false
	}
	var texts []string
	for _, p := range parts {
		switch p.Type {
		case "text":
			if p.Text != "" {
				texts = append(texts, p.Text)
			}
		case "tool_use":
			m.Tools = append(m.Tools, ToolCall{Name: p.Name, Brief: toolBrief(p.Input)})
		default:
			// tool_result / thinking / 其他：不展示
		}
	}
	// 用户消息里全是 tool_result 时没有文本，会被外层跳过
	m.Text = strings.Join(texts, "\n")
	return m, true
}

// toolBrief 从工具入参里挑一个最有信息量的字段当摘要。
func toolBrief(input json.RawMessage) string {
	var v map[string]any
	if json.Unmarshal(input, &v) != nil {
		return ""
	}
	for _, k := range []string{"command", "file_path", "pattern", "url", "description", "query", "path"} {
		if s, ok := v[k].(string); ok && s != "" {
			return truncate(s, 120)
		}
	}
	data, _ := json.Marshal(v)
	return truncate(string(data), 120)
}

// truncate 换行压成空格，超过 n 个字符截断。按字符（rune）数而不是字节：
// 一个汉字占 3 字节，按字节切会把汉字劈成两半，显示成乱码。
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ReadSession 读一个会话文件并解析成消息（给查看器）。rel 是相对项目目录的路径，禁止跳出目录。
func ReadSession(pid, rel string) ([]SessionMessage, error) {
	if strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") || strings.ContainsRune(rel, '\\') {
		return nil, errors.New(i18n.T("非法路径"))
	}
	cfg, _, b, err := openBinding(pid)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(cfg.ProjectsDir(), b.Dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	return parseSession(data), nil
}

// openBinding 按 pid 找到本机绑定。
func openBinding(pid string) (*config.Config, *state.State, *state.Binding, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	b := st.Bindings[pid]
	if b == nil {
		return nil, nil, nil, fmt.Errorf(i18n.T("项目未绑定: %s"), pid)
	}
	return cfg, st, b, nil
}
