package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

func TestLegacyGlobalRecordsDefaultToClaude(t *testing.T) {
	cfg := testutil.NewDevice(t, "legacy")
	run := time.Now().Format(runLayout)
	oldBackup := filepath.Join(BackupDir(), run, "_global", "plans", "old.md.local")
	if err := os.MkdirAll(filepath.Dir(oldBackup), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldBackup, []byte("旧备份\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := ListBackups()
	if err != nil || len(records) != 1 {
		t.Fatalf("读取旧备份失败：%+v, %v", records, err)
	}
	r := records[0]
	if r.Tool != config.ToolClaude || r.Category != config.ContentPlans || r.Rel != "old.md" {
		t.Fatalf("旧备份应按 Claude 解释：%+v", r)
	}
	target, err := backupTarget(cfg, &r)
	if err != nil || target != filepath.Join(cfg.ClaudeDir, "plans", "old.md") {
		t.Fatalf("旧备份恢复目标不对：%q, %v", target, err)
	}

	// 旧冲突 meta.json 没有 tool 字段，仍按 Claude 的 key 和路径处理。
	c := &Conflict{Key: profileFilePrefix + "plans/conflict.md", Category: config.ContentPlans,
		Rel: "conflict.md", Kind: ConflictFile, LocalPath: filepath.Join(cfg.ClaudeDir, "plans", "conflict.md")}
	if err := saveConflict(c, ConflictBlobs{}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConflict(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tool != config.ToolClaude {
		t.Fatalf("旧冲突应默认 Claude：%+v", loaded)
	}
}
