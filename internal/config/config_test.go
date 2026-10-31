package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// 旧版（阿里云 OSS 专用 SDK）写的 config.json，读进来自动变成 S3 兼容配置，指向同一个 bucket。
func TestLoadMigratesOldOSSConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CODE_SYNC_HOME", dir)
	old := `{
  "deviceId": "abc",
  "deviceName": "pc",
  "storage": {"type": "oss", "region": "cn-shanghai", "bucket": "my-bucket",
              "accessKeyId": "ak", "accessKeySecret": "sk", "prefix": "v1/"}
}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Storage
	if s.Type != storage.TypeS3 || s.Provider != storage.ProviderAliyun || s.Bucket != "my-bucket" ||
		s.Prefix != "v1/" || s.ResolvedEndpoint() != "https://s3.oss-cn-shanghai.aliyuncs.com" {
		t.Errorf("迁移结果不对: %+v，endpoint %s", s, s.ResolvedEndpoint())
	}
}

func TestContentPendingOnlyForCCSwitchOverlap(t *testing.T) {
	cfg := &Config{}
	for _, category := range ContentCategories {
		if cfg.ContentPending(category, false) {
			t.Errorf("CC Switch 关闭时 %s 不应等待选择负责人", category)
		}
		want := CCSwitchCategories[category]
		if got := cfg.ContentPending(category, true); got != want {
			t.Errorf("CC Switch 开启时 %s 的 pending = %v，想要 %v", category, got, want)
		}
	}
}

func TestContentPendingStopsAfterChoosingCCSwitch(t *testing.T) {
	cfg := &Config{}
	cfg.SetContentEnabled(ContentRules, false)
	if cfg.ContentPending(ContentRules, true) {
		t.Fatal("已经交给 CC Switch 的类别不应再反复提示选择负责人")
	}
}

func TestCodexDefaultsAndIndependentContentSwitches(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	cfg := &Config{Home: home, ClaudeDir: filepath.Join(home, ".claude")}
	cfg.ApplyDefaults()
	if want := filepath.Join(home, ".codex"); cfg.CodexDir != want {
		t.Fatalf("默认 CodexDir = %q，想要 %q", cfg.CodexDir, want)
	}
	custom := filepath.Join(home, "custom-codex")
	t.Setenv("CODEX_HOME", custom)
	cfg = &Config{Home: home}
	cfg.ApplyDefaults()
	if cfg.CodexDir != custom {
		t.Fatalf("CODEX_HOME 应决定 CodexDir，得到 %q", cfg.CodexDir)
	}

	cfg.SetContentEnabledFor(ToolCodex, ContentSkills, false)
	if cfg.ContentEnabledFor(ToolCodex, ContentSkills) {
		t.Fatal("Codex skills 应被关闭")
	}
	if !cfg.ContentEnabled(ContentSkills) || !cfg.ContentEnabledFor(ToolCodex, ContentRules) {
		t.Fatal("Codex skills 开关不应影响 Claude skills 或 Codex rules")
	}
	if !ContentCategoryKnownFor(ToolCodex, ContentRules) || ContentCategoryKnownFor(ToolCodex, ContentPlans) {
		t.Fatal("Codex 首阶段只应注册 rules 和 skills")
	}
}

func TestUpdateCCSwitchReenableClearsApproval(t *testing.T) {
	cfg := &Config{}
	cfg.UpdateCCSwitch(true)
	cfg.ContentApproved = map[string]bool{ContentRules: true, ContentSkills: true, ContentMCP: true}

	cfg.UpdateCCSwitch(false)
	if len(cfg.ContentApproved) != 3 {
		t.Fatal("CC Switch 关闭时不应提前清除 owner 选择")
	}
	for _, category := range ContentCategories {
		if cfg.ContentPending(category, false) {
			t.Errorf("CC Switch 关闭后 %s 不应 pending", category)
		}
	}

	if !cfg.UpdateCCSwitch(true) {
		t.Fatal("CC Switch 从关闭变为开启应报告配置变化")
	}
	if len(cfg.ContentApproved) != 0 {
		t.Fatalf("重新开启后旧批准应清空，还剩 %+v", cfg.ContentApproved)
	}
	for _, category := range []string{ContentRules, ContentSkills, ContentMCP} {
		if !cfg.ContentPending(category, true) {
			t.Errorf("重新开启后 %s 应重新暂停并等待选择负责人", category)
		}
	}
}
