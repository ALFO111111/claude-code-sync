package engine

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

func writeProfileFile(t *testing.T, d *device, rel, content string) string {
	t.Helper()
	abs := filepath.Join(d.cfg.ClaudeDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return abs
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeProfileCCSwitch(t *testing.T, d *device, enabled bool) {
	t.Helper()
	path := filepath.Join(d.cfg.Home, ".cc-switch", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"s3Sync":{"enabled":false,"autoSync":false}}`
	if enabled {
		content = `{"s3Sync":{"enabled":true,"autoSync":true}}`
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProfileFilesAcrossDevices(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	files := map[string]string{
		"CLAUDE.md":             "规则\n",
		"plans/p1.md":           "计划\n",
		"skills/demo/SKILL.md":  "技能\n",
		"agents/reviewer.md":    "代理\n",
		"commands/check.md":     "命令\n",
		"tasks/current.json":    "[{\"id\":\"t1\",\"title\":\"任务\"}]\n",
		"skills/synced/junk.md": "不重复同步\n",
	}
	for rel, content := range files {
		writeProfileFile(t, a, rel, content)
	}
	a.sync(t, Options{})
	b.sync(t, Options{})
	for rel, want := range files {
		if rel == "skills/synced/junk.md" {
			if _, err := os.Stat(filepath.Join(b.cfg.ClaudeDir, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
				t.Error("CC Switch 的 skills/synced 不应被本工具下载")
			}
			continue
		}
		got := readFile(t, filepath.Join(b.cfg.ClaudeDir, filepath.FromSlash(rel)))
		if got != want {
			t.Errorf("%s 下载内容不对：%q", rel, got)
		}
	}
	eb := b.sync(t, Options{})
	if eb.stats.Pushed+eb.stats.Pulled+eb.stats.Merged != 0 {
		t.Fatalf("内容没变时不应重复传输：%+v", eb.stats)
	}
}

func TestProfileCCSwitchPendingSkipsOnlyOverlap(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)

	writeProfileFile(t, a, "CLAUDE.md", "A 的规则\n")
	writeProfileFile(t, a, "skills/demo/SKILL.md", "A 的技能\n")
	writeProfileFile(t, a, "plans/from-a.md", "A 的计划\n")
	writeJSON(t, a.cfg.ClaudeStatePath(), map[string]any{
		"mcpServers": map[string]any{"from-a": map[string]any{"command": "a"}},
	})
	ea := a.sync(t, Options{})

	writeProfileFile(t, b, "CLAUDE.md", "B 的规则\n")
	writeProfileFile(t, b, "skills/demo/SKILL.md", "B 的技能\n")
	writeProfileFile(t, b, "plans/from-b.md", "B 的计划\n")
	writeJSON(t, b.cfg.ClaudeStatePath(), map[string]any{
		"mcpServers": map[string]any{"from-b": map[string]any{"command": "b"}},
	})
	writeProfileCCSwitch(t, b, true)

	paused := []struct {
		key  string
		path string
	}{
		{profileFilePrefix + "rules/CLAUDE.md", filepath.Join(b.cfg.ClaudeDir, "CLAUDE.md")},
		{profileFilePrefix + "skills/demo/SKILL.md", filepath.Join(b.cfg.ClaudeDir, "skills", "demo", "SKILL.md")},
		{profileMCPKey, b.cfg.ClaudeStatePath()},
	}
	remoteBefore := map[string]string{}
	localBefore := map[string]string{}
	for _, item := range paused {
		remoteBefore[item.key] = ea.readRemote(t, item.key)
		localBefore[item.key] = readFile(t, item.path)
	}

	eb := b.sync(t, Options{})
	for _, item := range paused {
		if got := eb.readRemote(t, item.key); got != remoteBefore[item.key] {
			t.Errorf("pending 类别不应上传 %s：云端变成 %q", item.key, got)
		}
		if got := readFile(t, item.path); got != localBefore[item.key] {
			t.Errorf("pending 类别不应下载 %s：本机变成 %q", item.key, got)
		}
		if _, ok := eb.st.Files[item.key]; ok {
			t.Errorf("pending 类别不应写 state：%s", item.key)
		}
		if _, err := os.Stat(basePath(item.key)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("pending 类别不应写 base：%s，Stat 错误 %v", item.key, err)
		}
	}

	planAKey := profileFilePrefix + "plans/from-a.md"
	planBKey := profileFilePrefix + "plans/from-b.md"
	if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "plans", "from-a.md")); got != "A 的计划\n" {
		t.Errorf("plans 不应被 CC Switch 暂停，下载得到 %q", got)
	}
	if got := eb.readRemote(t, planBKey); got != "B 的计划\n" {
		t.Errorf("plans 不应被 CC Switch 暂停，上传得到 %q", got)
	}
	for _, key := range []string{planAKey, planBKey} {
		if eb.st.Files[key] == nil {
			t.Errorf("正常同步的 plans 应写 state：%s", key)
		}
		if _, err := os.Stat(basePath(key)); err != nil {
			t.Errorf("正常同步的 plans 应写 base：%s: %v", key, err)
		}
	}
}

func TestProfileIndependentAndAutomaticMerge(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeProfileFile(t, a, "plans/shared.md", "一\n二\n三\n四\n")
	a.sync(t, Options{})
	b.sync(t, Options{})

	writeProfileFile(t, a, "plans/shared.md", "一\nA\n三\n四\n")
	writeProfileFile(t, b, "plans/shared.md", "一\n二\n三\nB\n")
	writeProfileFile(t, a, "agents/a.md", "A 的代理\n")
	writeProfileFile(t, b, "agents/b.md", "B 的代理\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	a.sync(t, Options{})
	for _, d := range []*device{a, b} {
		if got := readFile(t, filepath.Join(d.cfg.ClaudeDir, "plans/shared.md")); got != "一\nA\n三\nB\n" {
			t.Errorf("不同行应自动合并，得到 %q", got)
		}
		for _, rel := range []string{"agents/a.md", "agents/b.md"} {
			if _, err := os.Stat(filepath.Join(d.cfg.ClaudeDir, rel)); err != nil {
				t.Errorf("两边独立新增应取并集 %s: %v", rel, err)
			}
		}
	}
}

func TestProfileConflictKeepsLocal(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeProfileFile(t, a, "CLAUDE.md", "共同\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	writeProfileFile(t, a, "CLAUDE.md", "A 的修改\n")
	writeProfileFile(t, b, "CLAUDE.md", "B 的修改\n")
	a.sync(t, Options{})
	b.sync(t, Options{})

	if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "CLAUDE.md")); got != "B 的修改\n" {
		t.Fatalf("冲突时本机必须保持可用，得到 %q", got)
	}
	t.Setenv("CLAUDE_CODE_SYNC_HOME", b.home)
	list, err := ListConflicts()
	if err != nil || len(list) != 1 {
		t.Fatalf("应留下一个可处理冲突：%+v, %v", list, err)
	}
	blobs, err := ReadConflictBlobs(list[0].ID)
	if err != nil || string(blobs.Remote) != "A 的修改\n" || string(blobs.Local) != "B 的修改\n" {
		t.Fatalf("冲突应保存双方正文：%+v, %v", blobs, err)
	}
}

func TestProfileDeleteNeverPropagates(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	absA := writeProfileFile(t, a, "plans/keep.md", "保留\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if err := os.Remove(absA); err != nil {
		t.Fatal(err)
	}
	a.sync(t, Options{})
	b.sync(t, Options{})
	if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "plans/keep.md")); got != "保留\n" {
		t.Fatalf("A 删除不应影响 B：%q", got)
	}
	a.sync(t, Options{})
	if _, err := os.Stat(absA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("A 删除后不应自动复活：%v", err)
	}
	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	st, err := state.Load()
	if err != nil || !st.Files[profileFilePrefix+"plans/keep.md"].Deleted {
		t.Fatalf("本机删除应留下 tombstone：%+v, %v", st.Files[profileFilePrefix+"plans/keep.md"], err)
	}
	writeProfileFile(t, a, "plans/keep.md", "手动恢复\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "plans/keep.md")); got != "手动恢复\n" {
		t.Fatalf("手动恢复应重新参与同步：%q", got)
	}
}

func TestProfileDryIsReadOnly(t *testing.T) {
	a := newDevice(t, "devA", nil)
	writeProfileFile(t, a, "plans/dry.md", "只预览\n")
	a.cfg.ContentApproved = map[string]bool{config.ContentRules: true}
	if err := config.Save(a.cfg); err != nil {
		t.Fatal(err)
	}
	writeProfileCCSwitch(t, a, true) // Dry 会在内存里更新检测结果，但不能落盘
	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	e, err := New(true)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(state.Path())
	configBefore, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Sync(context.Background(), Options{Dry: true}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(state.Path())
	if !bytes.Equal(before, after) {
		t.Error("Dry 不应修改 state")
	}
	configAfter, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(configBefore, configAfter) {
		t.Error("Dry 不应修改 config（包括 CC Switch 检测状态和 owner 批准）")
	}
	objs, err := e.store.List(context.Background(), profileFilePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) > 0 {
		t.Error("Dry 不应上传文件")
	}
	for _, dir := range []string{"base", "conflicts", "backup"} {
		if _, err := os.Stat(filepath.Join(a.home, dir)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("Dry 不应创建 %s", dir)
		}
	}
}

func TestGlobalBackupRestoresToClaudeDir(t *testing.T) {
	a := newDevice(t, "devA", nil)
	abs := writeProfileFile(t, a, "plans/p.md", "旧版\n")
	a.sync(t, Options{})
	writeProfileFile(t, a, "plans/p.md", "本机新版\n")
	a.sync(t, Options{})
	ea := a.sync(t, Options{})
	ea.overwriteRemote(t, profileFilePrefix+"plans/p.md", "远端新版\n")
	a.sync(t, Options{})
	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	list, err := ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, r := range list {
		if r.Scope == "global" && r.Category == config.ContentPlans && r.Rel == "p.md" {
			id = r.ID
			break
		}
	}
	if id == "" {
		t.Fatalf("应列出全局备份：%+v", list)
	}
	if _, err := RestoreBackup(id); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, abs); got != "本机新版\n" {
		t.Fatalf("备份应恢复到原 Claude 路径，得到 %q", got)
	}
}
