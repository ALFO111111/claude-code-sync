package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

func writeContentCCSwitch(t *testing.T, home string, enabled bool) {
	t.Helper()
	path := filepath.Join(home, ".cc-switch", "settings.json")
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

func checkContentItem(t *testing.T, category string, wantEnabled, wantPending bool) {
	t.Helper()
	overview, err := LoadContent()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range overview.Items {
		if item.Category == category {
			if item.Enabled != wantEnabled || item.Pending != wantPending {
				t.Fatalf("%s 的 enabled/pending = %v/%v，想要 %v/%v", category, item.Enabled, item.Pending, wantEnabled, wantPending)
			}
			return
		}
	}
	t.Fatalf("内容总览缺少类别 %s", category)
}

func TestCodexContentItemsIgnoreCCSwitch(t *testing.T) {
	cfg := testutil.NewDevice(t, "dev-codex-content")
	writeContentCCSwitch(t, cfg.Home, true)
	overview, err := LoadContent()
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Items) != len(config.ContentCategories)+len(config.CodexContentCategories) {
		t.Fatalf("应返回 Claude 和 Codex 两组内容：%+v", overview.Items)
	}
	for _, item := range overview.Items {
		if item.Tool == config.ToolCodex && (!item.Enabled || item.Pending || item.CCOverlap) {
			t.Errorf("Codex 不应被 CC Switch 暂停：%+v", item)
		}
	}
	if err := SetToolContentEnabled(config.ToolCodex, config.ContentSkills, false); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentEnabledFor(config.ToolCodex, config.ContentSkills) || !got.ContentEnabled(config.ContentSkills) {
		t.Fatalf("Codex 开关应独立保存：%+v", got.ContentDisabled)
	}
	if err := SetToolContentEnabled(config.ToolCodex, config.ContentPlans, true); err == nil {
		t.Fatal("Codex 首阶段不应接受 plans 开关")
	}
}

func TestSetContentOwner(t *testing.T) {
	cfg := testutil.NewDevice(t, "dev-owner")
	writeContentCCSwitch(t, cfg.Home, true)

	for _, category := range []string{config.ContentRules, config.ContentSkills, config.ContentMCP} {
		t.Run(category, func(t *testing.T) {
			checkContentItem(t, category, false, true)

			if err := SetContentOwner(category, "sync"); err != nil {
				t.Fatal(err)
			}
			got, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !got.ContentApproved[category] || !got.ContentEnabled(category) {
				t.Fatalf("owner=sync 应批准并恢复本工具同步：%+v", got)
			}
			checkContentItem(t, category, true, false)

			if err := SetContentOwner(category, "ccswitch"); err != nil {
				t.Fatal(err)
			}
			got, err = config.Load()
			if err != nil {
				t.Fatal(err)
			}
			if got.ContentApproved[category] || got.ContentEnabled(category) {
				t.Fatalf("owner=ccswitch 应取消批准并禁用本工具同步：%+v", got)
			}
			checkContentItem(t, category, false, false)
		})
	}
}
