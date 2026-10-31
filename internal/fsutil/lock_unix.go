//go:build !windows

package fsutil

import (
	"errors"
	"os"
	"syscall"
)

// TryLockFile 不等待地给整个文件加独占锁；被别人锁着返回 false。
// 和 Rust 标准库 File::try_lock 是同一个系统调用（flock），所以能和 Codex 的写锁互斥。
func TryLockFile(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

// UnlockFile 释放 TryLockFile 加的锁。
func UnlockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// ProcessGone 确定这个进程已经不在了才返回 true；没权限查看等拿不准的情况返回 false。
func ProcessGone(pid int) bool {
	return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}
