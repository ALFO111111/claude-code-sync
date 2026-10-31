package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
)

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSettingsFieldMergePreservesLocalOwnership(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	settingsA := filepath.Join(a.cfg.ClaudeDir, "settings.json")
	settingsB := filepath.Join(b.cfg.ClaudeDir, "settings.json")
	writeJSON(t, settingsA, map[string]any{
		"language": "zh-CN", "autoMemoryEnabled": true, "includeCoAuthoredBy": true,
		"model": "model-A", "env": map[string]any{"TOKEN": "secret-A"}, "hooks": map[string]any{"Stop": []any{"A"}},
	})
	writeJSON(t, settingsB, map[string]any{
		"model": "model-B", "env": map[string]any{"TOKEN": "secret-B"},
		"permissions": map[string]any{"allow": []any{"Bash"}}, "cleanupPeriodDays": 30,
	})
	a.sync(t, Options{})
	b.sync(t, Options{})
	ma := readJSON(t, settingsA)
	mb := readJSON(t, settingsB)
	ma["language"] = "en"
	mb["includeCoAuthoredBy"] = false
	writeJSON(t, settingsA, ma)
	writeJSON(t, settingsB, mb)
	a.sync(t, Options{})
	b.sync(t, Options{})
	a.sync(t, Options{})

	for _, path := range []string{settingsA, settingsB} {
		m := readJSON(t, path)
		if m["language"] != "en" || m["includeCoAuthoredBy"] != false || m["autoMemoryEnabled"] != true {
			t.Errorf("不同白名单字段应自动合并：%+v", m)
		}
	}
	ma, mb = readJSON(t, settingsA), readJSON(t, settingsB)
	if ma["model"] != "model-A" || mb["model"] != "model-B" || ma["env"].(map[string]any)["TOKEN"] != "secret-A" || mb["env"].(map[string]any)["TOKEN"] != "secret-B" {
		t.Errorf("模型和秘密必须各留本机：A=%+v B=%+v", ma, mb)
	}
	if _, ok := ma["hooks"]; !ok {
		t.Error("A 的 hooks 不应被覆盖")
	}
	if _, ok := mb["permissions"]; !ok || mb["cleanupPeriodDays"] != float64(30) {
		t.Error("B 的 permissions 和 cleanupPeriodDays 不应被覆盖")
	}
}

func TestMCPPortableProjectionAndLocalSecrets(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	stateA, stateB := a.cfg.ClaudeStatePath(), b.cfg.ClaudeStatePath()
	writeJSON(t, stateA, map[string]any{
		"mcpServers": map[string]any{
			"safe":     map[string]any{"command": filepath.Join(a.cfg.Home, "bin", "tool.exe"), "args": []any{"--stdio"}, "env": map[string]any{"KEY": "secret-A"}},
			"unmapped": map[string]any{"command": `D:\unmapped\tool.exe`},
		},
		"private": "A",
	})
	writeJSON(t, stateB, map[string]any{"private": "B"})
	ea := a.sync(t, Options{})
	cloud := ea.readRemote(t, profileMCPKey)
	if strings.Contains(cloud, "secret-A") || strings.Contains(cloud, "unmapped") || strings.Contains(cloud, "env") {
		t.Fatalf("MCP 云端投影泄露了本机内容：%s", cloud)
	}
	b.sync(t, Options{})
	mb := readJSON(t, stateB)
	safe := mb["mcpServers"].(map[string]any)["safe"].(map[string]any)
	if safe["command"] != filepath.Join(b.cfg.Home, "bin", "tool.exe") || mb["private"] != "B" {
		t.Fatalf("MCP 路径应转换到 B，其他字段保留：%+v", mb)
	}
	safe["env"] = map[string]any{"KEY": "secret-B"}
	safe["headers"] = map[string]any{"Authorization": "local-B"}
	ma := readJSON(t, stateA)
	ma["mcpServers"].(map[string]any)["safe"].(map[string]any)["args"] = []any{filepath.Join(a.cfg.Home, "config.json")}
	writeJSON(t, stateA, ma)
	writeJSON(t, stateB, mb)
	a.sync(t, Options{})
	b.sync(t, Options{})
	mb = readJSON(t, stateB)
	safe = mb["mcpServers"].(map[string]any)["safe"].(map[string]any)
	if safe["args"].([]any)[0] != filepath.Join(b.cfg.Home, "config.json") {
		t.Fatalf("参数里的 home 路径也应转换：%+v", safe)
	}
	if safe["env"].(map[string]any)["KEY"] != "secret-B" || safe["headers"].(map[string]any)["Authorization"] != "local-B" {
		t.Fatalf("B 自己补的秘密必须保留：%+v", safe)
	}
}

func TestTasksMergeByStableID(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeProfileFile(t, a, "tasks/current.json", "[{\"id\":\"one\",\"title\":\"一\",\"done\":false},{\"id\":\"two\",\"title\":\"二\",\"done\":false}]\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	writeProfileFile(t, a, "tasks/current.json", "[{\"id\":\"one\",\"title\":\"A 修改\",\"done\":true},{\"id\":\"two\",\"title\":\"二\",\"done\":false}]\n")
	writeProfileFile(t, b, "tasks/current.json", "[{\"id\":\"one\",\"title\":\"一\",\"done\":false},{\"id\":\"two\",\"title\":\"B 修改\",\"done\":true}]\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	a.sync(t, Options{})
	for _, d := range []*device{a, b} {
		got := readFile(t, filepath.Join(d.cfg.ClaudeDir, "tasks", "current.json"))
		if !strings.Contains(got, "A 修改") || !strings.Contains(got, "B 修改") {
			t.Errorf("不同任务 ID 应按字段合并：%s", got)
		}
	}
}

func TestStructuredBackupRestoreKeepsUnmanagedFields(t *testing.T) {
	a := newDevice(t, "devA", nil)
	path := filepath.Join(a.cfg.ClaudeDir, "settings.json")
	writeJSON(t, path, map[string]any{"language": "zh-CN", "model": "local-model"})
	ea := a.sync(t, Options{})
	m := readJSON(t, path)
	m["language"] = "en"
	writeJSON(t, path, m)
	ea = a.sync(t, Options{})
	ea.overwriteRemote(t, profileSettingsKey, `{"version":1,"settings":{"language":"fr"}}`)
	a.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", a.home)
	list, err := ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range list {
		if r.Scope == "global" && r.Category == config.ContentSettings {
			if _, err := RestoreBackup(r.ID); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	m = readJSON(t, path)
	if m["language"] != "en" || m["model"] != "local-model" {
		t.Fatalf("结构化备份只恢复受管字段：%+v", m)
	}
}
