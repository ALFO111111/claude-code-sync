package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
)

// 锁有心跳：持有锁的进程还在跑（即使超过 15 分钟），别人不能把它当残留锁删掉。
func TestLockHeartbeatPreventsTakeover(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SYNC_HOME", t.TempDir())
	old := lockHeartbeatInterval
	lockHeartbeatInterval = 50 * time.Millisecond
	defer func() { lockHeartbeatInterval = old }()

	unlock, err := AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	// 把锁文件的修改时间改到 20 分钟前，模拟"跑了很久的同步"
	lockPath := filepath.Join(config.Dir(), "sync.lock")
	past := time.Now().Add(-20 * time.Minute)
	if err := os.Chtimes(lockPath, past, past); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // 等心跳把 mtime 刷回来

	if _, err := AcquireLock(); !errors.Is(err, ErrLocked) {
		t.Errorf("持有中的锁不应被抢走，得到 err=%v", err)
	}
}

// 真正的残留锁（没有进程维持心跳）还是要能被接管。
func TestStaleLockTakenOver(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SYNC_HOME", t.TempDir())
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(config.Dir(), "sync.lock")
	if err := os.WriteFile(lockPath, []byte("999"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-20 * time.Minute)
	os.Chtimes(lockPath, past, past)

	unlock, err := AcquireLock()
	if err != nil {
		t.Fatalf("残留锁应被接管: %v", err)
	}
	unlock()
}
