package claudeconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSettingsProjectionAndApplyDefaultDeny(t *testing.T) {
	full := []byte(`{
		"language":"zh-CN", "autoMemoryEnabled":true, "includeCoAuthoredBy":false,
		"model":"private-model", "env":{"TOKEN":"secret"}, "hooks":{"Stop":[]},
		"permissions":{"allow":["Bash"]}, "cleanupPeriodDays":30
	}`)
	projection, err := SettingsProjection(full)
	if err != nil {
		t.Fatal(err)
	}
	text := string(projection)
	for _, forbidden := range []string{"private-model", "secret", "hooks", "permissions", "cleanupPeriodDays"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("投影不应包含 %s：%s", forbidden, text)
		}
	}
	remote := []byte(`{"version":1,"settings":{"language":"en"},"model":"attacker","hooks":{}}`)
	merged, err := ApplySettings(full, remote)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(merged, &m); err != nil {
		t.Fatal(err)
	}
	if m["language"] != "en" || m["model"] != "private-model" {
		t.Fatalf("只应替换白名单字段：%+v", m)
	}
	if _, ok := m["autoMemoryEnabled"]; ok {
		t.Error("投影里删除受管字段时，本机对应字段也应删除")
	}
	if m["env"].(map[string]any)["TOKEN"] != "secret" || m["cleanupPeriodDays"] != float64(30) {
		t.Fatalf("非受管字段必须保留：%+v", m)
	}
}

func TestMCPProjectionStripsSecretsAndSkipsUnmappedPath(t *testing.T) {
	full := []byte(`{"mcpServers":{
		"safe":{"command":"C:\\Users\\a\\tool.exe","args":["--stdio"],"env":{"KEY":"secret"},"headers":{"Authorization":"secret"}},
		"unmapped":{"command":"D:\\private\\tool.exe"},
		"secretArg":{"command":"tool","args":["--api-key","secret"]}
	},"other":true}`)
	mapper := func(path string) (string, bool) {
		if strings.HasPrefix(path, `C:\Users\a\`) {
			return "${CLAUDE_SYNC_HOME}/" + strings.TrimPrefix(path, `C:\Users\a\`), true
		}
		return "", false
	}
	projection, skipped, err := MCPProjection(full, mapper)
	if err != nil {
		t.Fatal(err)
	}
	text := string(projection)
	if strings.Contains(text, "secret") || strings.Contains(text, "env") || strings.Contains(text, "headers") || strings.Contains(text, "unmapped") {
		t.Fatalf("秘密和不可移植项不应上传：%s", text)
	}
	if !strings.Contains(text, "${CLAUDE_SYNC_HOME}/tool.exe") || len(skipped) != 2 {
		t.Fatalf("安全项应转换路径，其余两项跳过：%s %+v", text, skipped)
	}
}

func TestApplyMCPPreservesLocalSecrets(t *testing.T) {
	full := []byte(`{"mcpServers":{"safe":{"command":"old","env":{"KEY":"local-secret"},"headers":{"X":"local"}}},"other":true}`)
	projection := []byte(`{"version":1,"mcpServers":{"safe":{"command":"new","args":["--stdio"],"env":{"KEY":"remote-secret"}}}}`)
	merged, skipped, err := ApplyMCP(full, projection, nil)
	if err != nil || len(skipped) != 0 {
		t.Fatal(err, skipped)
	}
	var m struct {
		Servers map[string]map[string]any `json:"mcpServers"`
		Other   bool                      `json:"other"`
	}
	if err := json.Unmarshal(merged, &m); err != nil {
		t.Fatal(err)
	}
	server := m.Servers["safe"]
	if server["command"] != "new" || server["env"].(map[string]any)["KEY"] != "local-secret" || !m.Other {
		t.Fatalf("应应用受管字段并保留本机秘密：%+v", m)
	}
	if strings.Contains(string(merged), "remote-secret") {
		t.Fatal("远端携带的秘密字段不能被应用")
	}
}

func TestRejectTrailingJSON(t *testing.T) {
	if _, err := SettingsProjection([]byte(`{} garbage`)); err == nil {
		t.Fatal("多余内容必须拒绝")
	}
	if _, err := SettingsProjection([]byte(`{} {}`)); err == nil {
		t.Fatal("第二个 JSON 值必须拒绝")
	}
}
