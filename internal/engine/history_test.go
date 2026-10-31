package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

// device 一台测试设备：切换到它（AGENT_SYNC_HOME 指过去）再打开引擎。
type device struct {
	cfg  *config.Config
	home string // AGENT_SYNC_HOME
}

func newDevice(t *testing.T, id string, cloudFrom *device) *device {
	t.Helper()
	cfg := testutil.NewDevice(t, id)
	if cloudFrom != nil {
		cfg.Storage = cloudFrom.cfg.Storage
		if err := config.Save(cfg); err != nil {
			t.Fatal(err)
		}
	}
	return &device{cfg: cfg, home: config.Dir()}
}

func (d *device) sync(t *testing.T, opts Options) *Engine {
	t.Helper()
	t.Setenv("AGENT_SYNC_HOME", d.home)
	e, err := New(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Sync(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	return e
}

func (d *device) historyPath() string { return filepath.Join(d.cfg.ClaudeDir, "history.jsonl") }

func histLine(display string, ts int64, project string) string {
	b, _ := json.Marshal(map[string]any{"display": display, "pastedContents": map[string]any{}, "timestamp": ts, "project": project})
	return string(b) + "\n"
}

func (d *device) writeHistory(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(d.historyPath(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (d *device) appendHistory(t *testing.T, content string) {
	t.Helper()
	f, err := os.OpenFile(d.historyPath(), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

// historyEntries 读本机历史：display → project。顺带检查没有重复行。
func (d *device) historyEntries(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(d.historyPath())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, l := range SplitLines(data) {
		var v struct{ Display, Project string }
		if err := json.Unmarshal(l, &v); err != nil {
			t.Fatalf("坏行 %q: %v", l, err)
		}
		if _, dup := out[v.Display]; dup {
			t.Fatalf("重复的历史: %s", v.Display)
		}
		out[v.Display] = v.Project
	}
	return out
}

// 两台设备的输入历史互相补齐，project 路径翻译成本机路径，不重复、不丢、已有内容不被改写。
func TestHistorySync(t *testing.T) {
	a := newDevice(t, "devA", nil)
	rootA := filepath.Join(a.cfg.Home, "proj")
	_, dirA := testutil.AddProject(t, a.cfg, "proj")
	testutil.WriteProjectFile(t, a.cfg, dirA, "s1.jsonl", sessionLine(rootA, "u1"))
	_, homeDir := testutil.AddProject(t, a.cfg, "")
	testutil.WriteProjectFile(t, a.cfg, homeDir, "s2.jsonl", sessionLine(a.cfg.Home, "u2"))
	a.writeHistory(t, histLine("A 在项目里", 1000, rootA)+histLine("A 在用户目录", 2000, a.cfg.Home))
	ea := a.sync(t, Options{})
	pidA := ""
	for pid, bd := range ea.st.Bindings {
		if bd.Root == rootA {
			pidA = pid
		}
	}

	// B：自己有一条历史，认领 A 的项目到不同的路径；用户目录自动对上
	b := newDevice(t, "devB", a)
	rootB := filepath.Join(b.cfg.Home, "work", "proj2")
	if err := os.MkdirAll(rootB, 0o700); err != nil {
		t.Fatal(err)
	}
	before := histLine("B 自己的", 1500, b.cfg.Home)
	b.writeHistory(t, before)
	eb := b.sync(t, Options{Claim: &Claim{PID: pidA, Root: rootB}})
	got := b.historyEntries(t)
	if len(got) != 3 || got["A 在项目里"] != rootB || got["A 在用户目录"] != b.cfg.Home || got["B 自己的"] != b.cfg.Home {
		t.Fatalf("B 的历史不对: %v", got)
	}
	if eb.Stats().History != 2 || eb.Stats().Pulled == 0 {
		t.Errorf("B 应补进 2 条历史: %+v", eb.Stats())
	}
	if data, _ := os.ReadFile(b.historyPath()); !strings.HasPrefix(string(data), before) {
		t.Errorf("已有内容不能被改写，只能往后追加:\n%s", data)
	}

	// 回到 A：拿到 B 的那条，路径翻译回 A 的
	ea = a.sync(t, Options{})
	got = a.historyEntries(t)
	if len(got) != 3 || got["B 自己的"] != a.cfg.Home || got["A 在项目里"] != rootA {
		t.Fatalf("A 的历史不对: %v", got)
	}
	// 再同步一次：什么都不该发生
	if ea = a.sync(t, Options{}); ea.Stats().History != 0 {
		t.Errorf("没变化时不应再补: %+v", ea.Stats())
	}

	// A 正写到一半（最后一行不完整）：这轮不往里追加，免得拼坏那一行
	a.appendHistory(t, `{"display":"写到一半`)
	b.appendHistory(t, histLine("B 新输入", 3000, rootB))
	b.sync(t, Options{})
	a.sync(t, Options{})
	if data, _ := os.ReadFile(a.historyPath()); !strings.HasSuffix(string(data), `{"display":"写到一半`) {
		t.Fatalf("写到一半时不该追加:\n%s", data)
	}
	// 写完了：下一轮补进来
	a.appendHistory(t, `","timestamp":2500,"project":"x"}`+"\n")
	a.sync(t, Options{})
	got = a.historyEntries(t)
	if got["B 新输入"] != rootA || got["写到一半"] != "x" || len(got) != 5 {
		t.Fatalf("写完后应补进: %v", got)
	}
	// B 也能拿到 A 那条
	b.sync(t, Options{})
	if got = b.historyEntries(t); len(got) != 5 {
		t.Fatalf("B 最终应有 5 条: %v", got)
	}
}

// 路径对照：最长的先匹配；不在任何项目下的路径原样不动；正斜杠、大小写都能认。
func TestTranslateRoot(t *testing.T) {
	roots := []rootMap{
		{from: `C:\Users\other\proj`, to: `D:\work\proj`},
		{from: `C:\Users\other`, to: `C:\Users\me`},
	}
	cases := map[string]string{
		`C:\Users\other\proj`:     `D:\work\proj`,
		`c:/users/other/proj/sub`: `D:\work\proj\sub`,
		`C:\Users\other`:          `C:\Users\me`,
		`C:\Users\other\notes`:    `C:\Users\me\notes`,
		`C:\Users\otherX`:         `C:\Users\otherX`,
		`E:\elsewhere`:            `E:\elsewhere`,
		`C:\Users\other\项目 (1)\a`: `C:\Users\me\项目 (1)\a`,
	}
	for in, want := range cases {
		if got := translateRoot(in, roots); got != want {
			t.Errorf("%s → %s，应为 %s", in, got, want)
		}
	}
}
