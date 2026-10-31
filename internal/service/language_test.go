package service

import (
	"os"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

// 向导阶段（还没初始化）切换语言：只改内存，不能凭空建出 config.json。
func TestSetLanguageBeforeInit(t *testing.T) {
	t.Cleanup(func() { i18n.Set(i18n.ZhCN) })
	t.Setenv("AGENT_SYNC_HOME", t.TempDir())
	if err := SetLanguage("en-US"); err != nil {
		t.Fatal(err)
	}
	if i18n.Lang() != i18n.En {
		t.Errorf("语言应已切到 en，实际 %s", i18n.Lang())
	}
	if _, err := os.Stat(config.Path()); !os.IsNotExist(err) {
		t.Errorf("没初始化时不应写 config.json: %v", err)
	}
}

// 已初始化：切换语言存进 config.json，下次启动 ApplyLanguage 恢复；空串 = 跟随系统。
func TestSetLanguagePersists(t *testing.T) {
	t.Cleanup(func() { i18n.Set(i18n.ZhCN) })
	testutil.NewDevice(t, "dev-a")
	if err := SetLanguage("en"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := config.Load(); cfg.Language != i18n.En {
		t.Errorf("config.json 里的语言 = %q，期望 en", cfg.Language)
	}
	i18n.Set(i18n.ZhCN) // 模拟重启
	ApplyLanguage()
	if got := GetLanguage(); got.Setting != i18n.En || got.Effective != i18n.En {
		t.Errorf("重启后应恢复 en: %+v", got)
	}
	if err := SetLanguage(""); err != nil {
		t.Fatal(err)
	}
	if got := GetLanguage(); got.Setting != "" || got.Effective != i18n.Detect() {
		t.Errorf("空串应跟随系统: %+v", got)
	}
}
