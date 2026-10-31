package service

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ALFO111111/agent-sync/internal/engine"
)

// 会话浏览：会话列表 + 完整对话查看器的解析（GUI 会话页用）。只读本地文件，不连云端。

// SessionInfo 会话列表里的一行。
type SessionInfo struct {
	Name    string `json:"name"` // 文件名（<uuid>.jsonl）
	Rel     string `json:"rel"`  // 相对项目目录的路径
	Size    int64  `json:"size"`
	Lines   int    `json:"lines"`            // 记录条数
	Mtime   string `json:"mtime"`            // 已格式化（Wails 绑定不认 time.Time）
	Preview string `json:"preview"`          // 首条用户消息摘要
	Title   string `json:"title"`            // 会话标题：用户起的名字 > Claude 起的名字 > 空（界面用摘要代替）
	Synced  bool   `json:"synced"`           // 已同步到云端
	ForkOf  string `json:"forkOf,omitempty"` // 这是分叉出来的副本：原会话的 ID
	Resume  bool   `json:"resume"`           // 能在 Claude Code 里继续（顶层会话）
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// summaryCache 会话列表的摘要缓存：路径 → 上次读到时的大小、修改时间和摘要。
// 每次打开列表都把所有会话整个读一遍太慢（会话文件动辄几 MB），没变的直接用缓存。
var summaryCache sync.Map

type summaryEntry struct {
	size    int64
	mtime   time.Time
	lines   int
	preview string
	title   string
}

func cachedSummary(path string, info fs.FileInfo) (lines int, preview, title string, ok bool) {
	if v, hit := summaryCache.Load(path); hit {
		if c := v.(summaryEntry); c.size == info.Size() && c.mtime.Equal(info.ModTime()) {
			return c.lines, c.preview, c.title, true
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", "", false
	}
	lines, preview, title = sessionSummary(data)
	summaryCache.Store(path, summaryEntry{info.Size(), info.ModTime(), lines, preview, title})
	return lines, preview, title, true
}

// sessionSummary 数记录条数，找首条用户消息当摘要，再找标题：
// 用户用 /rename 起的名字（custom-title）优先，其次 Claude 自动起的（ai-title），都取最后一次的。
func sessionSummary(data []byte) (lines int, preview, title string) {
	var custom, ai string
	for _, l := range engine.SplitLines(data) {
		lines++
		if bytes.Contains(l, []byte(`"type":"custom-title"`)) || bytes.Contains(l, []byte(`"type":"ai-title"`)) {
			var v struct {
				Type        string `json:"type"`
				CustomTitle string `json:"customTitle"`
				AITitle     string `json:"aiTitle"`
			}
			if json.Unmarshal(l, &v) == nil {
				if v.Type == "custom-title" && v.CustomTitle != "" {
					custom = v.CustomTitle
				} else if v.Type == "ai-title" && v.AITitle != "" {
					ai = v.AITitle
				}
			}
			continue
		}
		if preview == "" {
			if m, ok := parseMessageLine(l); ok && m.Role == "user" && m.Text != "" {
				preview = truncate(m.Text, 100)
			}
		}
	}
	if title = custom; title == "" {
		title = ai
	}
	return lines, preview, truncate(title, 100)
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

// isSessionID 顶层会话的文件名就是会话 ID（标准 UUID），Claude Code 的 --resume 认它。
func isSessionID(id string) bool { return len(id) == 36 && strings.Count(id, "-") == 4 }

// ---------- 跨项目搜索 ----------

// SearchHit 搜索结果：一个会话里有几条消息命中，以及第一处命中的上下文。
type SearchHit struct {
	PID     string `json:"pid"`
	Project string `json:"project"`
	Rel     string `json:"rel"`
	Title   string `json:"title"`
	Mtime   string `json:"mtime"`
	Count   int    `json:"count"`   // 命中的消息条数
	Snippet string `json:"snippet"` // 第一处命中前后的一段文字
}

// maxSearchHits 搜索最多返回多少个会话。
const maxSearchHits = 100

// snippet 命中处前后的一段文字，换行压成空格。i 是命中在小写文本里的字节下标，q 是小写关键词。
// strings.ToLower 逐个字符转换、不增减字符数，所以按字符数换算回原文的位置是准的。
func snippet(text string, i int, q string) string {
	r := []rune(text)
	at := utf8.RuneCountInString(strings.ToLower(text)[:i])
	from, to := max(0, at-30), min(len(r), at+utf8.RuneCountInString(q)+90)
	s := string(r[from:to])
	if from > 0 {
		s = "…" + s
	}
	if to < len(r) {
		s += "…"
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// ---------- 继续会话 ----------

// ResumeInfo 继续一个会话要做的事：在项目目录里运行 claude --resume / codex resume <会话 ID>。
type ResumeInfo struct {
	Dir     string `json:"dir"`
	Command string `json:"command"` // 可以直接粘贴到终端（PowerShell、Git Bash 都认）
}
