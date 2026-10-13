// Package logging 同步日志：写 sync.log（超过 5MB 轮转一次），非静默模式同时打到终端。
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

type Logger struct {
	quiet bool
	mu    sync.Mutex
}

// Path sync.log 的位置。
func Path() string { return filepath.Join(config.Dir(), "sync.log") }

// New quiet=true 时只写文件不打终端（hook、定时任务、GUI 用）。
func New(quiet bool) *Logger {
	if fi, err := os.Stat(Path()); err == nil && fi.Size() > 5<<20 {
		os.Rename(Path(), Path()+".1")
	}
	return &Logger{quiet: quiet}
}

func (l *Logger) Quiet() bool { return l.quiet }

// Printf 写一行日志。格式串是中文原文，按当前语言翻译后再填参数（见 internal/i18n）。
func (l *Logger) Printf(format string, args ...any) {
	msg := fmt.Sprintf(i18n.T(format), args...)
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.quiet {
		fmt.Println(msg)
	}
	os.MkdirAll(config.Dir(), 0o700)
	f, err := os.OpenFile(Path(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%d] %s\n", time.Now().Format("2006-01-02 15:04:05"), os.Getpid(), msg)
}
