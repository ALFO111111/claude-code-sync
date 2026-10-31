package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testHookCmd = `"C:\tools\agent-sync.exe" hook-event`

func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readSettingsText(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// hooks 安装：合并进 settings.json，不动已有配置和别的工具的 hook，重复安装不重复加。
func TestEnsureHooksMergesAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{
  "model": "opus",
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "other-tool"}]}]
  }
}`)

	if err := ensureHooks(dir, testHookCmd); err != nil {
		t.Fatal(err)
	}
	if ok, err := hooksConfigured(dir, testHookCmd); err != nil || !ok {
		t.Fatalf("安装后 hooksConfigured 应为 true: %v %v", ok, err)
	}
	s := readSettingsText(t, dir)
	if !strings.Contains(s, `"model": "opus"`) || !strings.Contains(s, "other-tool") {
		t.Errorf("已有配置或别的 hook 被弄丢了: %s", s)
	}

	// 再装一次：内容不变
	if err := ensureHooks(dir, testHookCmd); err != nil {
		t.Fatal(err)
	}
	if s2 := readSettingsText(t, dir); s2 != s {
		t.Errorf("重复安装不应改动文件:\n%s\n---\n%s", s, s2)
	}
}

// exe 换了位置：旧路径的 hook 被改成新路径，不会留两份。
func TestEnsureHooksUpdatesMovedExe(t *testing.T) {
	dir := t.TempDir()
	if err := ensureHooks(dir, `"D:\old\agent-sync.exe" hook-event`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := hooksConfigured(dir, testHookCmd); ok {
		t.Fatal("路径不同时 hooksConfigured 应为 false（首页据此提示/修复）")
	}
	if err := ensureHooks(dir, testHookCmd); err != nil {
		t.Fatal(err)
	}
	s := readSettingsText(t, dir)
	if strings.Contains(s, `D:\\old`) {
		t.Errorf("旧路径应被替换: %s", s)
	}
	if n := strings.Count(s, "hook-event"); n != 2 {
		t.Errorf("SessionStart、Stop 各一条，应共 2 条，得到 %d: %s", n, s)
	}
}

// settings.json 不存在时直接创建。
func TestEnsureHooksFreshFile(t *testing.T) {
	dir := t.TempDir()
	if ok, err := hooksConfigured(dir, testHookCmd); err != nil || ok {
		t.Fatalf("未安装时应为 false: %v %v", ok, err)
	}
	if err := ensureHooks(dir, testHookCmd); err != nil {
		t.Fatal(err)
	}
	if ok, _ := hooksConfigured(dir, testHookCmd); !ok {
		t.Error("安装后应为 true")
	}
}

// 卸载只删本工具的 hook，别的工具的 hook 和其他配置原样保留。
func TestRemoveHooksKeepsOthers(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, `{
  "model": "opus",
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "other-tool"}]}]
  }
}`)
	if err := ensureHooks(dir, testHookCmd); err != nil {
		t.Fatal(err)
	}
	if err := removeHooks(dir); err != nil {
		t.Fatal(err)
	}
	s := readSettingsText(t, dir)
	if strings.Contains(s, "hook-event") || strings.Contains(s, "SessionStart") {
		t.Errorf("本工具的 hook 应删干净（SessionStart 变空要一并删）: %s", s)
	}
	if !strings.Contains(s, "other-tool") || !strings.Contains(s, `"model": "opus"`) {
		t.Errorf("别的配置不应被删: %s", s)
	}
}

// 沙箱模式（测试都在沙箱里）绝不能往真实环境装 hooks / 定时任务。
func TestEnsureAutoSyncSkippedInSandbox(t *testing.T) {
	t.Setenv("AGENT_SYNC_HOME", t.TempDir())
	if err := EnsureAutoSync(); err != nil {
		t.Errorf("沙箱模式应直接跳过，得到 %v", err)
	}
}
