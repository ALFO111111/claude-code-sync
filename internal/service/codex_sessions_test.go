package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/state"
)

const codexTestID = "019ca82d-fc82-7a12-ae80-49205ee19601"

func codexRollout(cwd string) string {
	return strings.Join([]string{
		`{"timestamp":"2026-03-01T06:55:24Z","type":"session_meta","payload":{"id":"` + codexTestID + `","cwd":` + jsonStr(cwd) + `}}`,
		`{"timestamp":"2026-03-01T06:55:24Z","type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"权限说明"}]}}`,
		`{"timestamp":"2026-03-01T06:55:24Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions"}]}}`,
		`{"timestamp":"2026-03-01T06:55:24Z","type":"event_msg","payload":{"type":"user_message","message":"# Context from my IDE setup:\n\n## Active file: a.go\n\n## My request for Codex:\n帮我删掉没用的文件"}}`,
		`{"timestamp":"2026-03-01T06:55:46Z","type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{\"command\":\"rg --files frontend\"}"}}`,
		`{"timestamp":"2026-03-01T06:55:50Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"删好了"}]}}`,
		`{"timestamp":"2026-03-01T06:55:51Z","type":"event_msg","payload":{"type":"token_count"}}`,
	}, "\n") + "\n"
}

func jsonStr(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

// Codex 会话：只显示真正的用户输入（去掉编辑器上下文和 Codex 自己拼的说明）、AI 回复和工具调用。
func TestParseCodexSession(t *testing.T) {
	msgs := parseCodexSession([]byte(codexRollout(`D:\x`)))
	if len(msgs) != 3 {
		t.Fatalf("应有 3 条消息，得到 %+v", msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "帮我删掉没用的文件" {
		t.Errorf("用户输入不对: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || len(msgs[1].Tools) != 1 || msgs[1].Tools[0].Name != "shell_command" || !strings.Contains(msgs[1].Tools[0].Brief, "rg --files") {
		t.Errorf("工具调用不对: %+v", msgs[1])
	}
	if msgs[2].Text != "删好了" {
		t.Errorf("AI 回复不对: %+v", msgs[2])
	}
}

// Codex 会话按工作目录归到绑定项目；会话页、搜索、继续都能用；不在任何项目里的会话不显示。
func TestCodexSessionsByProject(t *testing.T) {
	cfg, pid := newDeviceWithSession(t, "devA", "")
	st, _ := state.Load()
	root := st.Bindings[pid].Root
	write := func(rel, content string) {
		p := filepath.Join(cfg.CodexDir, "sessions", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rel := "2026/03/01/rollout-2026-03-01T14-55-12-" + codexTestID + ".jsonl"
	write(rel, codexRollout(filepath.Join(root, "frontend")))
	write("2026/03/02/rollout-2026-03-02T10-00-00-019ca82d-0000-7000-8000-000000000000.jsonl", strings.Replace(codexRollout(`Z:\elsewhere`), codexTestID, "019ca82d-0000-7000-8000-000000000000", 1))
	if err := os.WriteFile(filepath.Join(cfg.CodexDir, "session_index.jsonl"), []byte(`{"id":"`+codexTestID+`","thread_name":"清理前端"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	projects, err := ListToolProjects(config.ToolCodex)
	if err != nil || len(projects) != 1 || projects[0].PID != pid || projects[0].Sessions != 1 {
		t.Fatalf("应只有一个项目、一个会话: %v %+v", err, projects)
	}
	list, err := ListToolSessions(config.ToolCodex, pid)
	if err != nil || len(list) != 1 || list[0].Rel != rel || list[0].Title != "清理前端" || list[0].Preview != "帮我删掉没用的文件" || !list[0].Resume {
		t.Fatalf("会话列表不对: %v %+v", err, list)
	}
	if msgs, err := ReadToolSession(config.ToolCodex, pid, rel); err != nil || len(msgs) != 3 {
		t.Fatalf("读会话: %v %+v", err, msgs)
	}
	if _, err := ReadToolSession(config.ToolCodex, pid, "../../config.toml"); err == nil {
		t.Error("不在会话列表里的文件不能读")
	}
	if hits, err := SearchToolSessions(config.ToolCodex, "没用的文件"); err != nil || len(hits) != 1 || hits[0].Title != "清理前端" {
		t.Fatalf("搜索: %v %+v", err, hits)
	}
	r, err := ToolResumeCommand(config.ToolCodex, pid, rel)
	if err != nil || r.Dir != root || !strings.HasSuffix(r.Command, "codex resume "+codexTestID) {
		t.Fatalf("继续命令不对（子目录不存在时在项目根目录里继续）: %v %+v", err, r)
	}

	tools, err := ListTools()
	if err != nil || len(tools) != 2 || tools[1].ID != config.ToolCodex || tools[1].SessionCount != 1 || tools[0].SessionCount != 1 {
		t.Fatalf("工具清单不对: %v %+v", err, tools)
	}
}
