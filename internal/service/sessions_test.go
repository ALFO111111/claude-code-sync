package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ALFO111111/agent-sync/internal/state"
)

// 会话查看器：jsonl 解析成用户/AI 消息序列，工具调用折叠为摘要，tool_result 等噪音行跳过。
func TestParseSession(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"summary","summary":"xx"}`, // 非对话行：跳过
		`{"type":"user","timestamp":"2026-10-11T10:00:00Z","message":{"role":"user","content":"帮我修个 bug"}}`,
		`{"type":"assistant","timestamp":"2026-10-11T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"我看看"},{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"跑测试"}}]}}`,
		`{"type":"user","timestamp":"2026-10-11T10:00:08Z","message":{"role":"user","content":[{"type":"tool_result","content":[{"type":"text","text":"ok"}]}]}}`, // 工具结果：跳过
		`{"type":"assistant","timestamp":"2026-10-11T10:00:10Z","message":{"role":"assistant","content":[{"type":"thinking","thinking":"嗯"}]}}`,                   // 纯思考：跳过
		`{"type":"assistant","timestamp":"2026-10-11T10:00:12Z","message":{"role":"assistant","content":[{"type":"text","text":"修好了"}]}}`,
		`这不是 JSON，解析失败的行要跳过`,
	}, "\n")

	msgs := parseSession([]byte(data))
	if len(msgs) != 3 {
		t.Fatalf("应解析出 3 条消息，得到 %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "帮我修个 bug" {
		t.Errorf("第 1 条应是用户文本: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "我看看" {
		t.Errorf("第 2 条应是 AI 文本: %+v", msgs[1])
	}
	if len(msgs[1].Tools) != 1 || msgs[1].Tools[0].Name != "Bash" || !strings.Contains(msgs[1].Tools[0].Brief, "go test") {
		t.Errorf("工具调用摘要不对: %+v", msgs[1].Tools)
	}
	if msgs[2].Text != "修好了" {
		t.Errorf("第 3 条: %+v", msgs[2])
	}
	if msgs[0].Time == "" {
		t.Errorf("消息应带时间")
	}
}

// 会话列表 + 查看器：只列顶层会话，带摘要和同步状态；查看器拦住路径穿越。
func TestListAndReadSessions(t *testing.T) {
	_, pid := newDeviceWithSession(t, "devA",
		`{"type":"user","timestamp":"2026-10-11T10:00:00Z","message":{"role":"user","content":"第一条消息内容"}}`+"\n")

	list, err := ListSessions(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("应只有 1 个顶层会话，得到 %d", len(list))
	}
	s := list[0]
	if s.Name != "a1.jsonl" || s.Lines != 2 {
		t.Errorf("会话信息不对: %+v", s)
	}
	if s.Preview != "第一条消息内容" {
		t.Errorf("摘要应是首条用户消息: %q", s.Preview)
	}
	if !s.Synced {
		t.Errorf("已同步的会话 Synced 应为 true")
	}

	msgs, err := ReadSession(pid, "a1.jsonl")
	if err != nil || len(msgs) != 1 || msgs[0].Text != "第一条消息内容" {
		t.Fatalf("ReadSession: %v %+v", err, msgs)
	}
	if _, err := ReadSession(pid, "../evil.jsonl"); err == nil {
		t.Errorf("路径穿越应报错")
	}
	if _, err := ListSessions("git:nope"); err == nil {
		t.Errorf("未绑定的项目应报错")
	}
}

// 截断按字符数，不能把汉字劈成两半（按字节切会出现乱码）。
func TestTruncateKeepsChineseIntact(t *testing.T) {
	s := strings.Repeat("中", 150)
	got := truncate(s, 100)
	if !utf8.ValidString(got) {
		t.Fatalf("截断结果不是合法 UTF-8: %q", got)
	}
	if got != strings.Repeat("中", 100)+"…" {
		t.Errorf("应保留 100 个汉字加省略号，得到 %d 个字符", utf8.RuneCountInString(got))
	}
	if truncate("短", 100) != "短" {
		t.Error("没超长不应截断")
	}
}

// 空结果要返回 [] 而不是 nil（JSON null 会让前端 .length 报错）；
// 绑定了但本机会话目录还不存在（刚认领、还没拉到东西）时返回空列表，不报错。
func TestListSessionsEmpty(t *testing.T) {
	cfg, pid := newDeviceWithSession(t, "devA", "")
	st, _ := state.Load()
	if err := os.RemoveAll(filepath.Join(cfg.ProjectsDir(), st.Bindings[pid].Dir)); err != nil {
		t.Fatal(err)
	}
	list, err := ListSessions(pid)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("目录不存在应返回空列表: %v %#v", err, list)
	}
	if msgs := parseSession(nil); msgs == nil {
		t.Error("空会话应返回 [] 而不是 nil")
	}
}

// 标题：用户起的名字（custom-title）优先，其次 Claude 起的（ai-title），都取最后一次的。
func TestSessionSummaryTitle(t *testing.T) {
	data := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"第一句话"}}`,
		`{"type":"ai-title","aiTitle":"旧的自动标题"}`,
		`{"type":"ai-title","aiTitle":"新的自动标题"}`,
	}, "\n")
	lines, preview, title := sessionSummary([]byte(data))
	if lines != 3 || preview != "第一句话" || title != "新的自动标题" {
		t.Errorf("得到 %d %q %q", lines, preview, title)
	}
	_, _, title = sessionSummary([]byte(data + "\n" + `{"type":"custom-title","customTitle":"我起的名字"}` + "\n" + `{"type":"ai-title","aiTitle":"更晚的自动标题"}`))
	if title != "我起的名字" {
		t.Errorf("用户起的名字应优先，得到 %q", title)
	}
}

// 跨项目搜索：不分大小写，带命中条数和上下文；一个字不搜。
func TestSearchSessions(t *testing.T) {
	_, pid := newDeviceWithSession(t, "devA",
		`{"type":"user","timestamp":"2026-10-11T10:00:00Z","message":{"role":"user","content":"帮我看看 Login 页面的报错"}}`+"\n"+
			`{"type":"assistant","timestamp":"2026-10-11T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"login 的问题在这里"}]}}`+"\n")
	hits, err := SearchSessions("LOGIN")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].PID != pid || hits[0].Count != 2 || !strings.Contains(hits[0].Snippet, "Login 页面") {
		t.Fatalf("搜索结果不对: %+v", hits)
	}
	if hits, _ := SearchSessions("找不到的词"); len(hits) != 0 {
		t.Errorf("没命中应返回空: %+v", hits)
	}
	if hits, _ := SearchSessions("帮"); len(hits) != 0 {
		t.Errorf("一个字不搜: %+v", hits)
	}
}

func TestSnippet(t *testing.T) {
	text := strings.Repeat("前", 50) + "关键词" + strings.Repeat("后", 200)
	s := snippet(text, strings.Index(text, "关键词"), "关键词")
	if !strings.HasPrefix(s, "…") || !strings.HasSuffix(s, "…") || !strings.Contains(s, "关键词") || !utf8.ValidString(s) {
		t.Errorf("上下文截取不对: %q", s)
	}
	if got := snippet("ÀB关键", strings.Index(strings.ToLower("ÀB关键"), "关键"), "关键"); got != "ÀB关键" {
		t.Errorf("转小写后字节数会变的字符也要对得上，得到 %q", got)
	}
}

// 只有顶层会话（文件名是会话 ID）能在 Claude Code 里继续；命令在项目目录里执行。
func TestResumeCommand(t *testing.T) {
	_, pid := newDeviceWithSession(t, "devA", "")
	if _, err := ResumeCommand(pid, "a1.jsonl"); err == nil {
		t.Error("文件名不是会话 ID 时不能继续")
	}
	id := "11111111-2222-4333-8444-555555555555"
	r, err := ResumeCommand(pid, id+".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(r.Command, "claude --resume "+id) || !strings.Contains(r.Command, `cd "`+r.Dir+`"`) {
		t.Errorf("命令不对: %+v", r)
	}
}
