package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/state"
)

const codexTestID = "01a1598e-6ac0-7100-8829-e5df37b63797"

func codexRel(id string) string { return "2026/10/31/rollout-2026-10-31T10-00-00-" + id + ".jsonl" }

func codexLine(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b) + "\n"
}

// writeRollout 写一份最小的 Codex 会话：session_meta + 若干条消息（每条都带 cwd，用来检查路径翻译）。
func writeRollout(t *testing.T, d *device, id, cwd string, msgs ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(codexLine(map[string]any{"type": "session_meta", "payload": map[string]any{"id": id, "session_id": id, "cwd": cwd, "runtime_workspace_roots": []string{cwd}}}))
	for _, m := range msgs {
		b.WriteString(codexLine(map[string]any{"type": "turn_context", "payload": map[string]any{"cwd": cwd, "text": m}}))
	}
	abs := filepath.Join(d.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(id)))
	if err := fsutil.WriteFileAtomic(abs, []byte(b.String())); err != nil {
		t.Fatal(err)
	}
	return abs
}

func appendRollout(t *testing.T, abs, cwd string, msgs ...string) {
	t.Helper()
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, m := range msgs {
		f.WriteString(codexLine(map[string]any{"type": "turn_context", "payload": map[string]any{"cwd": cwd, "text": m}}))
	}
}

// rolloutTexts 会话里的消息文字（按顺序）和出现过的全部 cwd。
func rolloutTexts(t *testing.T, abs string) (texts []string, cwds map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	cwds = map[string]bool{}
	for _, l := range SplitLines(data) {
		var v struct {
			Type    string `json:"type"`
			Payload struct {
				ID, Cwd, Text string
			} `json:"payload"`
		}
		if err := json.Unmarshal(l, &v); err != nil {
			t.Fatalf("坏行 %q: %v", l, err)
		}
		cwds[v.Payload.Cwd] = true
		if v.Type == "turn_context" {
			texts = append(texts, v.Payload.Text)
		}
	}
	return texts, cwds
}

func codexFiles(t *testing.T, d *device) []string {
	t.Helper()
	var out []string
	root := filepath.Join(d.cfg.CodexDir, "sessions")
	filepath.WalkDir(root, func(p string, de os.DirEntry, err error) error {
		if err == nil && !de.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func TestCodexSessionAcrossDevicesAndContinue(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeRollout(t, a, codexTestID, a.cfg.Home, "A1")
	writeRollout(t, a, "01a1598e-0000-7000-8000-000000000001", a.cfg.Home, "另一个")
	os.WriteFile(filepath.Join(a.cfg.CodexDir, "state_5.sqlite"), []byte("不同步"), 0o600)

	a.sync(t, Options{})
	b.sync(t, Options{}) // B 第一次：先拿到 A 的设备信息，对上用户目录
	eb := b.sync(t, Options{})
	absB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(codexTestID)))
	texts, cwds := rolloutTexts(t, absB)
	if strings.Join(texts, ",") != "A1" || !cwds[b.cfg.Home] || cwds[a.cfg.Home] {
		t.Fatalf("B 应拿到翻译成 B 用户目录的会话：%v %v", texts, cwds)
	}
	if _, err := os.Stat(filepath.Join(b.cfg.CodexDir, "state_5.sqlite")); !errors.Is(err, os.ErrNotExist) {
		t.Error("SQLite 不应同步")
	}
	if n := len(codexFiles(t, b)); n != 2 {
		t.Fatalf("B 应有 2 个会话，得到 %d", n)
	}
	if eb = b.sync(t, Options{}); eb.stats.Pushed+eb.stats.Pulled+eb.stats.Forked != 0 {
		t.Fatalf("从 A 拉下来、B 没改过的会话不应重新上传：%+v", eb.stats)
	}

	// B 接着聊 → A 只追加缺的行，A 原有的内容一个字节都不变
	before, _ := os.ReadFile(absA)
	appendRollout(t, absB, b.cfg.Home, "B2", "B3")
	b.sync(t, Options{})
	ea := a.sync(t, Options{})
	texts, cwds = rolloutTexts(t, absA)
	if strings.Join(texts, ",") != "A1,B2,B3" || !cwds[a.cfg.Home] || cwds[b.cfg.Home] {
		t.Fatalf("A 应追加 B 续写的内容并翻译回 A 的路径：%v %v", texts, cwds)
	}
	after, _ := os.ReadFile(absA)
	if !bytes.HasPrefix(after, before) {
		t.Fatal("A 原有内容不应被改写（只能追加）")
	}
	if ea.stats.Pulled != 1 {
		t.Fatalf("A 应拉取 1 个会话：%+v", ea.stats)
	}
	if ea = a.sync(t, Options{}); ea.stats.Pushed+ea.stats.Pulled != 0 {
		t.Fatalf("A 追加完后不应再传：%+v", ea.stats)
	}
}

func TestCodexSessionDivergedKeepsBoth(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeRollout(t, a, codexTestID, a.cfg.Home, "共同")
	a.sync(t, Options{})
	b.sync(t, Options{})
	b.sync(t, Options{})
	absB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(codexTestID)))

	appendRollout(t, absA, a.cfg.Home, "A 续写")
	appendRollout(t, absB, b.cfg.Home, "B 续写")
	a.sync(t, Options{})
	eb := b.sync(t, Options{})
	ea := a.sync(t, Options{})
	if eb.stats.Forked != 1 || ea.stats.Forked != 1 {
		t.Fatalf("两边各应另存一份对方的版本：A %+v B %+v", ea.stats, eb.stats)
	}
	for _, d := range []*device{a, b} {
		files := codexFiles(t, d)
		if len(files) != 2 {
			t.Fatalf("%s 应有原会话 + 对方的分叉：%v", d.cfg.DeviceID, files)
		}
	}
	// 本机的原会话不动
	if texts, _ := rolloutTexts(t, absA); strings.Join(texts, ",") != "共同,A 续写" {
		t.Fatalf("A 自己的会话不应被改：%v", texts)
	}
	if texts, _ := rolloutTexts(t, absB); strings.Join(texts, ",") != "共同,B 续写" {
		t.Fatalf("B 自己的会话不应被改：%v", texts)
	}
	forkB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(b.st().CodexForks["devA/"+codexRel(codexTestID)]))
	if texts, _ := rolloutTexts(t, forkB); strings.Join(texts, ",") != "共同,A 续写" {
		t.Fatalf("B 上的分叉应是 A 的版本：%v", texts)
	}
	data, _ := os.ReadFile(forkB)
	if bytes.Contains(data, []byte(codexTestID)) {
		t.Fatal("分叉会话里不应再出现原会话 ID")
	}

	// 稳定：再同步不来回覆盖
	for i := 0; i < 2; i++ {
		ea, eb = a.sync(t, Options{}), b.sync(t, Options{})
		if s := ea.stats.Pushed + ea.stats.Forked + eb.stats.Pushed + eb.stats.Forked; s != 0 {
			t.Fatalf("分叉之后应稳定：A %+v B %+v", ea.stats, eb.stats)
		}
	}
	// A 再续写自己的原会话 → B 追加到那份分叉里
	appendRollout(t, absA, a.cfg.Home, "A 又一句")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if texts, _ := rolloutTexts(t, forkB); strings.Join(texts, ",") != "共同,A 续写,A 又一句" {
		t.Fatalf("A 后续的内容应追加进 B 上的分叉：%v", texts)
	}
	if n := len(codexFiles(t, b)); n != 2 {
		t.Fatalf("不应再多出分叉：%d", n)
	}
}

// st 读这台设备落盘的状态。
func (d *device) st() *state.State {
	os.Setenv("AGENT_SYNC_HOME", d.home)
	st, _ := state.Load()
	return st
}

func TestCodexSessionSkipsWhileCodexHoldsLock(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeRollout(t, a, codexTestID, a.cfg.Home, "1")
	a.sync(t, Options{})
	b.sync(t, Options{})
	b.sync(t, Options{})
	absB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(codexTestID)))

	// B 上的 Codex 正开着这个会话（持有 thread-writer-locks/<ID>.lock）
	lockPath := filepath.Join(b.cfg.CodexDir, "thread-writer-locks", codexTestID+".lock")
	os.MkdirAll(filepath.Dir(lockPath), 0o700)
	lf, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := fsutil.TryLockFile(lf); !ok || err != nil {
		t.Fatalf("测试里拿锁失败：%v %v", ok, err)
	}
	appendRollout(t, absA, a.cfg.Home, "2")
	a.sync(t, Options{})
	before, _ := os.ReadFile(absB)
	b.sync(t, Options{})
	if got, _ := os.ReadFile(absB); !bytes.Equal(got, before) {
		t.Fatal("Codex 正在用这个会话时不应往里写")
	}
	fsutil.UnlockFile(lf)
	lf.Close()
	b.sync(t, Options{})
	if texts, _ := rolloutTexts(t, absB); strings.Join(texts, ",") != "1,2" {
		t.Fatalf("Codex 关掉会话后应补上：%v", texts)
	}
}

func TestCodexSessionDeleteNotRestoredAndSwitch(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeRollout(t, a, codexTestID, a.cfg.Home, "1")
	a.sync(t, Options{})
	b.sync(t, Options{})
	b.sync(t, Options{})
	absB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(codexTestID)))
	if err := os.Remove(absB); err != nil {
		t.Fatal(err)
	}
	b.sync(t, Options{})
	appendRollout(t, absA, a.cfg.Home, "2")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if _, err := os.Stat(absB); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("B 删掉（或 Codex 归档挪走）的会话不应被拉回")
	}
	if _, err := os.Stat(absA); err != nil {
		t.Fatal("B 删除不应影响 A")
	}

	// 关掉 Codex 会话同步：本机新会话不上传
	t.Setenv("AGENT_SYNC_HOME", a.home)
	a.cfg.SetContentEnabledFor(config.ToolCodex, config.ContentSessions, false)
	config.Save(a.cfg)
	writeRollout(t, a, "01a1598e-0000-7000-8000-0000000000aa", a.cfg.Home, "关掉后写的")
	ea := a.sync(t, Options{})
	if ea.stats.Pushed != 0 {
		t.Fatalf("关掉后不应上传：%+v", ea.stats)
	}
}

func TestCodexSessionWaitsForProjectBinding(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	projA := filepath.Join(a.cfg.Home, "proj")
	os.MkdirAll(projA, 0o700)
	writeRollout(t, a, codexTestID, projA, "项目里聊的")
	a.sync(t, Options{})
	b.sync(t, Options{})
	b.sync(t, Options{})
	if n := len(codexFiles(t, b)); n != 0 {
		t.Fatal("项目在 B 上还没认领时不应拉下来（路径没法翻译）")
	}
	marked := false
	for k, s := range b.st().Files {
		if strings.HasPrefix(k, codexSessionPrefix+"devA/") && strings.HasPrefix(s.Hash, codexUnboundMark) {
			marked = true
		}
	}
	if !marked {
		t.Fatal("没对上的版本应记下来，绑定没变时不用每轮都重新下载")
	}
	pid := ""
	for p := range a.st().Bindings {
		if strings.HasPrefix(p, "local:") {
			pid = p
		}
	}
	if pid == "" {
		t.Fatalf("A 应为 Codex 会话所在目录建立项目绑定：%+v", a.st().Bindings)
	}
	projB := filepath.Join(b.cfg.Home, "work", "proj")
	os.MkdirAll(projB, 0o700)
	b.sync(t, Options{Claim: &Claim{PID: pid, Root: projB}})
	absB := filepath.Join(b.cfg.CodexDir, "sessions", filepath.FromSlash(codexRel(codexTestID)))
	texts, cwds := rolloutTexts(t, absB)
	if strings.Join(texts, ",") != "项目里聊的" || !cwds[projB] {
		t.Fatalf("认领后应拉下来并翻译成 B 的项目路径：%v %v", texts, cwds)
	}
}

func TestCodexSessionDryAndUnsafeKeys(t *testing.T) {
	a := newDevice(t, "devA", nil)
	writeRollout(t, a, codexTestID, a.cfg.Home, "1")
	t.Setenv("AGENT_SYNC_HOME", a.home)
	e, err := New(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Sync(context.Background(), Options{Dry: true}); err != nil {
		t.Fatal(err)
	}
	if objs, _ := e.store.List(context.Background(), codexSessionPrefix); len(objs) != 0 {
		t.Fatalf("Dry 不应上传：%v", objs)
	}
	for _, rel := range []string{"../x/rollout-a.jsonl", `2026\rollout-a.jsonl`, "C:/rollout-a.jsonl", "2026/10/31/notes.jsonl", "2026/10/31/rollout-a.txt"} {
		if codexRelOK(rel) {
			t.Errorf("不安全或不是会话的路径应被拒绝：%q", rel)
		}
	}
	if !codexRelOK(codexRel(codexTestID)) {
		t.Error("正常会话路径应被接受")
	}
	if id := codexForkID(codexTestID, "devB"); !isUUID(id) || id[:13] != codexTestID[:13] || id[14] != '7' || id == codexForkID(codexTestID, "devC") {
		t.Errorf("分叉 ID 应保留时间前缀、是 v7、且按设备区分：%s", id)
	}
}
