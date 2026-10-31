package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/service"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/testutil"
)

// GetStatus 带上 GUI 自己知道的"是否在同步"。
func TestGetStatusReportsSyncing(t *testing.T) {
	testutil.NewDevice(t, "devA")
	a := NewApp()
	if s := a.GetStatus(); !s.Initialized || s.Syncing {
		t.Fatalf("状态不对: %+v", s)
	}
	a.syncing.Store(true)
	if !a.GetStatus().Syncing {
		t.Error("同步中 Syncing 应为 true")
	}
}

// 同步进行中：后台同步（立即同步/文件监听）不排队，记一笔、这轮结束后补跑一轮；
// 用户操作（认领/忽略）排队等上一轮结束再跑；跑完后标志复位。
func TestRunSyncQueueOrDrop(t *testing.T) {
	testutil.NewDevice(t, "devA")
	a := NewApp()
	a.syncMu.Lock() // 模拟一轮同步正在进行

	// 后台触发：不排队，记一笔等这轮结束补跑
	if err := a.runSync(service.SyncRequest{}, false); err != errBusy {
		t.Errorf("后台同步忙时应返回 errBusy，得到 %v", err)
	}
	if !a.rerun.Load() {
		t.Error("忙时的后台触发应记下来")
	}

	// 用户操作：排队等锁，不报错
	done := make(chan error, 1)
	go func() { done <- a.runSync(service.SyncRequest{IgnorePID: "x"}, true) }()
	select {
	case <-done:
		t.Fatal("上一轮没结束就跑了，没有排队")
	case <-time.After(200 * time.Millisecond):
	}
	a.syncMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if a.syncing.Load() {
		t.Error("同步结束后 syncing 应复位")
	}
	if a.rerun.Load() {
		t.Error("忙时被放弃的后台触发，应在这轮结束后补跑掉")
	}
}

func TestClaimProjectNeedsDirectory(t *testing.T) {
	testutil.NewDevice(t, "devA")
	if err := NewApp().ClaimProject("git:x", ""); err == nil {
		t.Error("没选目录应报错")
	}
}

func TestCodexWatchersAttachAfterDirectoriesAppear(t *testing.T) {
	cfg := testutil.NewDevice(t, "dev-watch")
	a := NewApp()
	a.startWatcher()
	defer a.shutdown(nil)
	if len(a.watchers) != 14 {
		t.Fatalf("应注册 Claude、Codex 和会话监听，实际 %d 个", len(a.watchers))
	}
	for _, w := range a.watchers {
		w.Debounce = 30 * time.Millisecond
		w.MaxWait = time.Second
	}

	// 启动时 .codex 和 .agents 都不存在；之后同时创建两个规则文件和 skills 子目录。
	if err := os.MkdirAll(cfg.CodexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENTS.md", "AGENTS.override.md"} {
		if err := os.WriteFile(filepath.Join(cfg.CodexDir, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	skill := filepath.Join(cfg.Home, ".agents", "skills", "demo", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"tools/codex/files/rules/AGENTS.md",
		"tools/codex/files/rules/AGENTS.override.md",
		"tools/codex/files/skills/demo/SKILL.md",
	}
	deadline := time.Now().Add(6 * time.Second)
	for {
		st, err := state.Load()
		if err == nil {
			found := 0
			for _, key := range want {
				if st.Files[key] != nil {
					found++
				}
			}
			if found == len(want) {
				a.syncMu.Lock() // 等后台同步彻底结束，再让 TempDir 清理云端目录。
				a.syncMu.Unlock()
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("Codex 目录出现后监听未触发完整同步：%+v", st.Files)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
