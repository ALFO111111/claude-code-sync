package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

var ErrLocked = i18n.Err("另一个同步正在进行")

// lockHeartbeatInterval 持有锁期间刷新锁文件 mtime 的间隔（测试可调小）。
// 没有心跳的话，一次超过 15 分钟的真实同步会被当成残留锁，导致两个同步并发跑。
var lockHeartbeatInterval = time.Minute

// AcquireLock 防止多个同步同时跑（hook、定时任务可能撞车）。超过 15 分钟没心跳的锁视为残留。
func AcquireLock() (func(), error) {
	path := filepath.Join(config.Dir(), "sync.lock")
	if err := os.MkdirAll(config.Dir(), 0o700); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%d", os.Getpid())
			f.Close()
			done := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				t := time.NewTicker(lockHeartbeatInterval)
				defer t.Stop()
				for {
					select {
					case <-done:
						return
					case now := <-t.C:
						os.Chtimes(path, now, now)
					}
				}
			}()
			return func() { close(done); wg.Wait(); os.Remove(path) }, nil
		}
		fi, serr := os.Stat(path)
		if serr != nil || time.Since(fi.ModTime()) < 15*time.Minute {
			break
		}
		os.Remove(path)
	}
	return nil, ErrLocked
}
