package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/state"
)

func writeCodexFile(t *testing.T, d *device, rel, content string) string {
	t.Helper()
	var abs string
	if strings.HasPrefix(rel, "skills/") {
		abs = filepath.Join(d.cfg.Home, ".agents", filepath.FromSlash(rel))
	} else {
		abs = filepath.Join(d.cfg.CodexDir, filepath.FromSlash(rel))
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return abs
}

func codexPath(d *device, rel string) string {
	if strings.HasPrefix(rel, "skills/") {
		return filepath.Join(d.cfg.Home, ".agents", filepath.FromSlash(rel))
	}
	return filepath.Join(d.cfg.CodexDir, filepath.FromSlash(rel))
}

func TestCodexFilesAcrossDevices(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	files := map[string]string{
		"AGENTS.md":              "Codex 规则\n",
		"AGENTS.override.md":     "本机覆盖规则\n",
		"skills/demo/SKILL.md":   "个人技能\n",
		"skills/demo/nested.txt": "嵌套文件\n",
	}
	for rel, content := range files {
		writeCodexFile(t, a, rel, content)
	}
	writeCodexFile(t, a, "config.toml", "不同步的配置\n")
	writeCodexFile(t, a, "sessions/rollout.jsonl", "不同步的会话\n")
	a.sync(t, Options{})
	writeProfileCCSwitch(t, b, true) // CC Switch 只暂停 Claude，不能影响 Codex。
	b.sync(t, Options{})
	for rel, want := range files {
		if got := readFile(t, codexPath(b, rel)); got != want {
			t.Errorf("%s 下载内容不对：%q", rel, got)
		}
	}
	for _, rel := range []string{"config.toml", "sessions/rollout.jsonl"} {
		if _, err := os.Stat(codexPath(b, rel)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("首阶段不应同步 %s", rel)
		}
	}
	eb := b.sync(t, Options{})
	if eb.stats.Pushed+eb.stats.Pulled+eb.stats.Merged != 0 {
		t.Fatalf("内容没变时不应重复传输：%+v", eb.stats)
	}
	objs, err := eb.store.List(context.Background(), profileFilePrefixFor(config.ToolCodex))
	if err != nil || len(objs) != len(files) {
		t.Fatalf("云端应只有四个 Codex 普通文件：%+v, %v", objs, err)
	}
	for _, obj := range objs {
		if strings.Contains(obj.Key, "config.toml") || strings.Contains(obj.Key, "sessions") {
			t.Errorf("不支持的对象被上传：%s", obj.Key)
		}
	}
}

func TestCodexIndependentAndAutomaticMerge(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeCodexFile(t, a, "AGENTS.md", "一\n二\n三\n四\n")
	a.sync(t, Options{})
	b.sync(t, Options{})

	writeCodexFile(t, a, "AGENTS.md", "一\nA\n三\n四\n")
	writeCodexFile(t, b, "AGENTS.md", "一\n二\n三\nB\n")
	writeCodexFile(t, a, "skills/a/SKILL.md", "A 的技能\n")
	writeCodexFile(t, b, "skills/b/SKILL.md", "B 的技能\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	a.sync(t, Options{})
	for _, d := range []*device{a, b} {
		if got := readFile(t, codexPath(d, "AGENTS.md")); got != "一\nA\n三\nB\n" {
			t.Errorf("不同行应自动合并，得到 %q", got)
		}
		for _, rel := range []string{"skills/a/SKILL.md", "skills/b/SKILL.md"} {
			if _, err := os.Stat(codexPath(d, rel)); err != nil {
				t.Errorf("两边独立新增应取并集 %s: %v", rel, err)
			}
		}
	}
}

func makeCodexConflict(t *testing.T) (*device, *device, Conflict) {
	t.Helper()
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeCodexFile(t, a, "skills/demo/SKILL.md", "共同\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	writeCodexFile(t, a, "skills/demo/SKILL.md", "A\n")
	writeCodexFile(t, b, "skills/demo/SKILL.md", "B\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", b.home)
	list, err := ListConflicts()
	if err != nil || len(list) != 1 {
		t.Fatalf("准备 Codex 冲突失败：%+v %v", list, err)
	}
	if list[0].Tool != config.ToolCodex || list[0].Key != profileFileKey(config.ToolCodex, "skills", "demo/SKILL.md") {
		t.Fatalf("冲突应记录 Codex 工具和完整云端 key：%+v", list[0])
	}
	if got := readFile(t, codexPath(b, "skills/demo/SKILL.md")); got != "B\n" {
		t.Fatalf("冲突期间本机文件应保持可用：%q", got)
	}
	return a, b, list[0]
}

func TestResolveCodexConflict(t *testing.T) {
	for _, choice := range []string{ResolveLocal, ResolveRemote, ResolveMerged, ResolveBoth} {
		t.Run(choice, func(t *testing.T) {
			a, b, c := makeCodexConflict(t)
			if err := ResolveConflict(context.Background(), c.ID, choice, []byte("手工合并\n")); err != nil {
				t.Fatal(err)
			}
			if list, _ := ListConflicts(); len(list) != 0 {
				t.Fatalf("解决后应清除冲突：%+v", list)
			}
			a.sync(t, Options{})
			b.sync(t, Options{})
			for _, d := range []*device{a, b} {
				entries, err := os.ReadDir(filepath.Dir(codexPath(d, "skills/demo/SKILL.md")))
				if err != nil {
					t.Fatal(err)
				}
				var contents []string
				for _, entry := range entries {
					contents = append(contents, readFile(t, filepath.Join(filepath.Dir(codexPath(d, "skills/demo/SKILL.md")), entry.Name())))
				}
				switch choice {
				case ResolveBoth:
					if len(entries) != 2 || !strings.Contains(strings.Join(contents, ""), "A\n") || !strings.Contains(strings.Join(contents, ""), "B\n") {
						t.Fatalf("双方内容都应保留并继续同步：%q", contents)
					}
				default:
					want := "A\n"
					if choice == ResolveLocal {
						want = "B\n"
					} else if choice == ResolveMerged {
						want = "手工合并\n"
					}
					if len(contents) != 1 || contents[0] != want {
						t.Fatalf("解决结果不对：%q，想要 %q", contents, want)
					}
				}
			}
		})
	}
}

func TestCodexDeleteRestoreAndCategorySwitch(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeCodexFile(t, a, "skills/keep/SKILL.md", "保留\n")
	writeCodexFile(t, a, "AGENTS.md", "规则一\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if err := os.Remove(absA); err != nil {
		t.Fatal(err)
	}
	a.sync(t, Options{})
	b.sync(t, Options{})
	if got := readFile(t, codexPath(b, "skills/keep/SKILL.md")); got != "保留\n" {
		t.Fatalf("A 删除不应影响 B：%q", got)
	}
	a.sync(t, Options{})
	if _, err := os.Stat(absA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("A 删除后不应自动复活：%v", err)
	}
	t.Setenv("AGENT_SYNC_HOME", a.home)
	st, err := state.Load()
	if err != nil || !st.Files[profileFileKey(config.ToolCodex, "skills", "keep/SKILL.md")].Deleted {
		t.Fatalf("Codex 删除应留下 tombstone：%+v, %v", st.Files, err)
	}
	if err := RestoreToolContentItem(context.Background(), config.ToolCodex, "skills", "keep/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, absA); got != "保留\n" {
		t.Fatalf("手动恢复失败：%q", got)
	}

	// 只关闭 Codex skills：Claude skills 和 Codex rules 都不受影响。
	t.Setenv("AGENT_SYNC_HOME", b.home)
	b.cfg.SetContentEnabledFor(config.ToolCodex, config.ContentSkills, false)
	if err := config.Save(b.cfg); err != nil {
		t.Fatal(err)
	}
	writeCodexFile(t, a, "skills/keep/SKILL.md", "关闭后不应下载\n")
	writeCodexFile(t, a, "AGENTS.md", "规则二\n")
	writeProfileFile(t, a, "skills/claude/SKILL.md", "Claude 技能\n")
	ea := a.sync(t, Options{})
	if got := ea.readRemote(t, profileFileKey(config.ToolCodex, "rules", "AGENTS.md")); got != "规则二\n" {
		t.Fatalf("A 应先上传 Codex rules：%q", got)
	}
	eb := b.sync(t, Options{})
	if eb.stats.Pulled == 0 {
		t.Fatalf("B 本轮应下载独立的 Codex rules：%+v", eb.stats)
	}
	if got := readFile(t, codexPath(b, "skills/keep/SKILL.md")); got != "保留\n" {
		t.Fatalf("关闭的 Codex skills 不应下载：%q", got)
	}
	if got := readFile(t, codexPath(b, "AGENTS.md")); got != "规则二\n" {
		t.Fatalf("Codex rules 应独立同步：%q", got)
	}
	if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "skills", "claude", "SKILL.md")); got != "Claude 技能\n" {
		t.Fatalf("Claude skills 的开关不应受 Codex 影响：%q", got)
	}

	writeProfileCCSwitch(t, b, true)
	writeCodexFile(t, a, "AGENTS.md", "规则三\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if got := readFile(t, codexPath(b, "AGENTS.md")); got != "规则三\n" {
		t.Fatalf("CC Switch 开启不应暂停 Codex：%q", got)
	}
}

func TestCodexDryIsReadOnly(t *testing.T) {
	a := newDevice(t, "devA", nil)
	writeCodexFile(t, a, "AGENTS.md", "只预览\n")
	t.Setenv("AGENT_SYNC_HOME", a.home)
	e, err := New(true)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(state.Path())
	if err := e.Sync(context.Background(), Options{Dry: true}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(state.Path())
	if !bytes.Equal(before, after) {
		t.Error("Dry 不应修改 state")
	}
	objs, err := e.store.List(context.Background(), profileFilePrefixFor(config.ToolCodex))
	if err != nil || len(objs) != 0 {
		t.Errorf("Dry 不应上传 Codex 文件：%+v, %v", objs, err)
	}
	for _, dir := range []string{"base", "conflicts", "backup"} {
		if _, err := os.Stat(filepath.Join(a.home, dir)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Dry 不应创建 %s", dir)
		}
	}
}

func TestCodexBackupRestore(t *testing.T) {
	a := newDevice(t, "devA", nil)
	abs := writeCodexFile(t, a, "AGENTS.md", "旧版\n")
	a.sync(t, Options{})
	writeCodexFile(t, a, "AGENTS.md", "本机新版\n")
	a.sync(t, Options{})
	ea := a.sync(t, Options{})
	ea.overwriteRemote(t, profileFileKey(config.ToolCodex, "rules", "AGENTS.md"), "远端新版\n")
	a.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", a.home)
	list, err := ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, r := range list {
		if r.Scope == "global" && r.Tool == config.ToolCodex && r.Category == config.ContentRules && r.Rel == "AGENTS.md" {
			id = r.ID
			if !strings.Contains(r.File, "_global/codex/rules/") {
				t.Errorf("Codex 备份路径应带工具名：%s", r.File)
			}
			break
		}
	}
	if id == "" {
		t.Fatalf("应列出 Codex 全局备份：%+v", list)
	}
	if _, err := RestoreBackup(id); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, abs); got != "本机新版\n" {
		t.Fatalf("备份应恢复到 CodexDir，得到 %q", got)
	}
}

func TestCodexRekeyAcrossDevices(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeCodexFile(t, a, "AGENTS.md", "换密钥前的规则\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	oldKey := crypt.DeriveKey("test-pass", crypt.KeySalt)
	newKey := crypt.DeriveKey("new-pass", crypt.KeySalt)
	t.Setenv("AGENT_SYNC_HOME", a.home)
	if err := ChangeKey(context.Background(), oldKey, newKey, nil); err != nil {
		t.Fatal(err)
	}
	writeCodexFile(t, a, "AGENTS.md", "换密钥后的规则\n")
	a.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", b.home)
	if err := AdoptKey(context.Background(), newKey); err != nil {
		t.Fatal(err)
	}
	b.sync(t, Options{})
	if got := readFile(t, codexPath(b, "AGENTS.md")); got != "换密钥后的规则\n" {
		t.Fatalf("改口令后 Codex 文件仍应可同步：%q", got)
	}
}

func TestCodexRejectsUnsafeRemotePathsAndSkipsSymlinks(t *testing.T) {
	for _, rel := range []string{"../escape.md", `..\escape.md`, "C:/escape.md", "/tmp/escape.md", "//server/share/escape.md"} {
		if !skipProfileRel(config.ToolCodex, config.ContentSkills, rel) {
			t.Errorf("异常云端路径应被拒绝：%q", rel)
		}
	}
	a := newDevice(t, "devA", nil)
	e := a.sync(t, Options{})
	for _, key := range []string{
		profileFilePrefixFor(config.ToolCodex) + "skills/../escape.md",
		profileFilePrefixFor(config.ToolCodex) + "rules/not-allowed.md",
	} {
		e.overwriteRemote(t, key, "不应落地\n")
	}
	target := writeCodexFile(t, a, "outside.md", "链接目标\n")
	link := filepath.Join(a.cfg.Home, ".agents", "skills", "linked.md")
	if err := os.Symlink(target, link); err != nil {
		t.Logf("当前 Windows 不允许创建符号链接，跳过链接部分：%v", err)
	} else {
		defer os.Remove(link)
	}
	a.sync(t, Options{})
	if _, err := os.Stat(filepath.Join(a.cfg.Home, ".agents", "escape.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("云端 ../ 路径不应写到目录外：%v", err)
	}
	if _, err := os.Stat(codexPath(a, "not-allowed.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("云端未注册规则文件不应落地：%v", err)
	}
	objs, err := e.store.List(context.Background(), profileFilePrefixFor(config.ToolCodex))
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objs {
		if strings.HasSuffix(obj.Key, "linked.md") || strings.HasSuffix(obj.Key, "outside.md") {
			t.Errorf("符号链接或 CodexDir 任意文件不应上传：%s", obj.Key)
		}
	}
}

func TestCodexLegacySkillsSkipSystem(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	for rel, text := range map[string]string{"skills/mine/SKILL.md": "旧位置的技能\n", "skills/.system/builtin/SKILL.md": "Codex 自带\n"} {
		p := filepath.Join(a.cfg.CodexDir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(text), 0o600)
	}
	a.sync(t, Options{})
	b.sync(t, Options{})
	if got := readFile(t, filepath.Join(b.cfg.CodexDir, "skills", "mine", "SKILL.md")); got != "旧位置的技能\n" {
		t.Fatalf("~/.codex/skills 里的个人技能应同步：%q", got)
	}
	if _, err := os.Stat(filepath.Join(b.cfg.CodexDir, "skills", ".system")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(".system 是 Codex 自带的，不应同步")
	}
}
