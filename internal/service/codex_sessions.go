package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/state"
)

// codexSessions Codex 的会话不按项目分目录：都在 $CODEX_HOME/sessions/年/月/日/rollout-*.jsonl，
// 第一行 session_meta 里写着工作目录，按它归到包含它的绑定项目（最长的根优先）。
type codexSessions struct{}

// codexMetaCache 每个会话文件的第一行（会话 ID、工作目录）：按路径 + 大小 + 修改时间缓存，
// 打开会话页时不用把几百个文件都重新读一遍。
var codexMetaCache sync.Map

type codexMetaEntry struct {
	size    int64
	mtime   time.Time
	id, cwd string
}

func codexMeta(abs string, fi fs.FileInfo) (id, cwd string) {
	if v, ok := codexMetaCache.Load(abs); ok {
		if e := v.(codexMetaEntry); e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
			return e.id, e.cwd
		}
	}
	id, cwd = engine.ReadCodexMeta(abs)
	codexMetaCache.Store(abs, codexMetaEntry{size: fi.Size(), mtime: fi.ModTime(), id: id, cwd: cwd})
	return id, cwd
}

func (codexSessions) files(cfg *config.Config, st *state.State) ([]sessionFile, error) {
	root := filepath.Join(cfg.CodexDir, "sessions")
	var out []sessionFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			return nil // 读不了的子目录跳过，不影响别的
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil || !engine.CodexRelOK(filepath.ToSlash(rel)) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		_, cwd := codexMeta(p, fi)
		if b := bindingFor(st, cwd); b != nil {
			out = append(out, sessionFile{b: b, rel: filepath.ToSlash(rel), abs: p})
		}
		return nil
	})
	return out, err
}

// bindingFor 包含这个工作目录的绑定项目（最长的根优先）；没有返回 nil。
func bindingFor(st *state.State, cwd string) *state.Binding {
	if cwd == "" {
		return nil
	}
	var best *state.Binding
	for _, b := range st.Bindings {
		if within(b.Root, cwd) && (best == nil || len(b.Root) > len(best.Root)) {
			best = b
		}
	}
	return best
}

func (c codexSessions) info(cfg *config.Config, st *state.State, f sessionFile) (SessionInfo, bool) {
	fi, err := os.Stat(f.abs)
	if err != nil {
		return SessionInfo{}, false
	}
	id, _ := codexMeta(f.abs, fi)
	s := SessionInfo{Name: filepath.Base(f.rel), Rel: f.rel, Size: fi.Size(), Mtime: fmtTime(fi.ModTime()), Resume: isSessionID(id)}
	s.Lines, s.Preview = codexSummary(f.abs, fi)
	s.Title = codexTitles(cfg)[id]
	if fs := st.Files[engine.CodexSessionKey(cfg.DeviceID, f.rel)]; fs != nil && !fs.Deleted {
		s.Synced = true
	}
	for from, local := range st.CodexForks {
		if local == f.rel {
			// from 是 "<对方设备ID>/<对方的相对路径>"，文件名最后 36 个字符是原会话 ID
			s.ForkOf = strings.TrimSuffix(from, ".jsonl")
			if len(s.ForkOf) >= 36 {
				s.ForkOf = s.ForkOf[len(s.ForkOf)-36:]
			}
			// Synced 照上面的规则：分叉也会作为本机会话上传
			break
		}
	}
	return s, true
}

// codexSummary 行数和第一条用户消息（缓存同 Claude 的摘要缓存）。
func codexSummary(abs string, fi fs.FileInfo) (lines int, preview string) {
	if v, ok := summaryCache.Load(abs); ok {
		if e := v.(summaryEntry); e.size == fi.Size() && e.mtime.Equal(fi.ModTime()) {
			return e.lines, e.preview
		}
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return 0, ""
	}
	msgs := parseCodexSession(data)
	for _, m := range msgs {
		if m.Role == "user" && m.Text != "" {
			preview = truncate(strings.Join(strings.Fields(m.Text), " "), 120)
			break
		}
	}
	lines = len(engine.SplitLines(data))
	summaryCache.Store(abs, summaryEntry{size: fi.Size(), mtime: fi.ModTime(), lines: lines, preview: preview})
	return lines, preview
}

// codexTitles 用户在 Codex 里给会话起的名字：$CODEX_HOME/session_index.jsonl，每行 {"id","thread_name"}，后写的为准。
func codexTitles(cfg *config.Config) map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(cfg.CodexDir, "session_index.jsonl"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var v struct {
			ID         string `json:"id"`
			ThreadName string `json:"thread_name"`
		}
		if json.Unmarshal(sc.Bytes(), &v) == nil && v.ID != "" && v.ThreadName != "" {
			out[v.ID] = v.ThreadName
		}
	}
	return out
}

func (codexSessions) parse(data []byte) []SessionMessage { return parseCodexSession(data) }

// parseCodexSession 只取"人说了什么、AI 回了什么、用了什么工具"：
//
//	event_msg / user_message            用户输入（VS Code 插件会在前面附带编辑器上下文，去掉）
//	response_item / message(assistant)  AI 的回复
//	response_item / function_call、custom_tool_call  工具调用
//
// response_item 里的 user / developer 消息是 Codex 自己拼的说明（AGENTS.md、权限说明），不展示。
func parseCodexSession(data []byte) []SessionMessage {
	out := []SessionMessage{}
	for _, l := range engine.SplitLines(data) {
		var v struct {
			Timestamp string `json:"timestamp"`
			Type      string `json:"type"`
			Payload   struct {
				Type      string `json:"type"`
				Role      string `json:"role"`
				Message   string `json:"message"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
				Input     string `json:"input"`
				Content   []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"payload"`
		}
		if json.Unmarshal(l, &v) != nil {
			continue
		}
		var tm string
		if t, err := time.Parse(time.RFC3339Nano, v.Timestamp); err == nil {
			tm = t.Local().Format("01-02 15:04:05")
		}
		p := v.Payload
		switch {
		case v.Type == "event_msg" && p.Type == "user_message":
			if text := codexUserText(p.Message); text != "" {
				out = append(out, SessionMessage{Role: "user", Time: tm, Text: text})
			}
		case v.Type == "response_item" && p.Type == "message" && p.Role == "assistant":
			var texts []string
			for _, c := range p.Content {
				if c.Text != "" {
					texts = append(texts, c.Text)
				}
			}
			if len(texts) > 0 {
				out = append(out, SessionMessage{Role: "assistant", Time: tm, Text: strings.Join(texts, "\n")})
			}
		case v.Type == "response_item" && (p.Type == "function_call" || p.Type == "custom_tool_call"):
			brief := toolBrief(json.RawMessage(p.Arguments))
			if brief == "" && p.Input != "" {
				brief = truncate(strings.SplitN(p.Input, "\n", 2)[0], 120)
			}
			call := ToolCall{Name: p.Name, Brief: brief}
			// 连着的工具调用并到上一条 AI 消息里，和 Claude 的显示方式一致
			if n := len(out); n > 0 && out[n-1].Role == "assistant" {
				out[n-1].Tools = append(out[n-1].Tools, call)
			} else {
				out = append(out, SessionMessage{Role: "assistant", Time: tm, Tools: []ToolCall{call}})
			}
		}
	}
	return out
}

// codexUserText VS Code 插件发的消息是"# Context from my IDE setup: …… ## My request for Codex:\n<真正的输入>"，只留真正的输入。
func codexUserText(s string) string {
	const marker = "## My request for Codex:"
	if i := strings.LastIndex(s, marker); i >= 0 {
		s = s[i+len(marker):]
	}
	return strings.TrimSpace(s)
}

func (codexSessions) resume(cfg *config.Config, f sessionFile) (string, []string, error) {
	fi, err := os.Stat(f.abs)
	if err != nil {
		return "", nil, err
	}
	id, cwd := codexMeta(f.abs, fi)
	if !isSessionID(id) {
		return "", nil, errors.New(i18n.T("只有顶层会话能继续"))
	}
	dir := cwd
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dir = f.b.Root // 会话所在的子目录本机没有，就在项目根目录里继续
	}
	return dir, []string{"codex", "resume", id}, nil
}
