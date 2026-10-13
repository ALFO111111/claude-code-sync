package gui

import (
	"testing"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/service"
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
