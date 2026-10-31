package state

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// 拿着锁的进程被强制结束 / 崩溃了：锁文件里的进程号已经不在，马上接管，不用等 15 分钟。
// 进程还在、或锁文件刚建还没写进程号，都不能接管。
func TestLockOfDeadProcessTakenOverImmediately(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SYNC_HOME", t.TempDir())
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(config.Dir(), "sync.lock")

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dead := exec.Command(exe, "-test.run=^$") // 起一个马上退出的进程，拿到一个确定已经不在的进程号
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name    string
		content string
		taken   bool
	}{
		{"进程已不在", strconv.Itoa(dead.Process.Pid), true},
		{"进程还在", strconv.Itoa(os.Getppid()), false},
		{"刚建还没写进程号", "", false},
	} {
		if err := os.WriteFile(lockPath, []byte(c.content), 0o600); err != nil {
			t.Fatal(err)
		}
		unlock, err := AcquireLock()
		if c.taken {
			if err != nil {
				t.Fatalf("%s：应马上接管，得到 %v", c.name, err)
			}
			unlock()
		} else if !errors.Is(err, ErrLocked) {
			t.Fatalf("%s：不应接管，得到 err=%v", c.name, err)
		}
	}
}
