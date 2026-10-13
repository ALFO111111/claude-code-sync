// Package watch 文件监听器：盯着会话目录，文件"消停"后回调通知同步。
//
// 这是 Dropbox / Syncthing 的标准做法（监听器为主、定时扫描兜底）：
// Claude Code 聊天时会不停往 .jsonl 追加内容，每次写入都同步太浪费，
// 所以等它停笔一小会儿（防抖）再同步；要是一直聊个不停，到 MaxWait 也强制同步一次。
// 同步引擎自己下载写文件也会触发事件——没关系，那轮同步会发现没有新变化，
// 不写任何文件，循环自然终止（最多空转一轮）。
package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/logging"
)

const (
	DefaultDebounce = 10 * time.Second // 停笔多久后触发同步
	DefaultMaxWait  = 2 * time.Minute  // 一直写个不停时，最多憋这么久
)

// Watcher 递归监听 root 目录。onChange 在防抖结束后被调用（在新 goroutine 里）。
type Watcher struct {
	root     string
	onChange func()

	Debounce time.Duration
	MaxWait  time.Duration

	only string // 非空：只盯 root 下这一个文件（NewFile），不递归

	fw     *fsnotify.Watcher
	stopCh chan struct{}
	once   sync.Once
}

// NewFile 只盯一个文件（如 ~/.claude/settings.json），变化消停 2 秒后回调。
// 监听的是它所在的目录、按文件名过滤：很多程序保存时"写临时文件再改名"，直接盯文件本身改名后就失效了。
func NewFile(path string, onChange func()) *Watcher {
	w := New(filepath.Dir(path), onChange)
	w.only = filepath.Base(path)
	w.Debounce = 2 * time.Second
	return w
}

func New(root string, onChange func()) *Watcher {
	return &Watcher{
		root:     root,
		onChange: onChange,
		Debounce: DefaultDebounce,
		MaxWait:  DefaultMaxWait,
		stopCh:   make(chan struct{}),
	}
}

// Start 开始监听（后台 goroutine）。目录不存在返回错误。
func (w *Watcher) Start() error {
	if _, err := os.Stat(w.root); err != nil {
		return err
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fw = fw
	if w.only != "" {
		if err := fw.Add(w.root); err != nil {
			fw.Close()
			return err
		}
		go w.loop()
		return nil
	}
	// 已存在的子目录全部加上监听（会话目录下还有 workflows/、tool-results/ 等）
	err = filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 个别目录进不去不耽误整体
		}
		if d.IsDir() {
			_ = w.fw.Add(path)
		}
		return nil
	})
	if err != nil {
		fw.Close()
		return err
	}
	go w.loop()
	return nil
}

// Stop 停止监听。可重复调用。
func (w *Watcher) Stop() {
	w.once.Do(func() {
		close(w.stopCh)
		if w.fw != nil {
			w.fw.Close()
		}
	})
}

func (w *Watcher) loop() {
	var timer *time.Timer
	var timerC <-chan time.Time
	var firstEvent time.Time
	fire := func() {
		if timer != nil {
			timer.Stop()
		}
		timerC = nil
		go w.onChange()
	}
	for {
		select {
		case <-w.stopCh:
			if timer != nil {
				timer.Stop()
			}
			return
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			// 原子写的临时文件（.ccs-tmp-*）不算变化，等 rename 成正式文件那次事件
			if strings.Contains(filepath.Base(ev.Name), fsutil.TempPrefix) {
				continue
			}
			if w.only != "" && !strings.EqualFold(filepath.Base(ev.Name), w.only) {
				continue
			}
			// 新建的子目录要补监听，不然里面的文件变化就漏了
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = w.fw.Add(ev.Name)
				}
			}
			now := time.Now()
			if timerC == nil {
				firstEvent = now
			}
			if now.Sub(firstEvent) >= w.MaxWait {
				fire() // 写个不停的（长对话），到上限就同步
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.Debounce)
			} else {
				timer.Reset(w.Debounce)
			}
			timerC = timer.C
		case <-timerC:
			fire()
		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			logging.New(true).Printf("文件监听出错: %v", err)
		}
	}
}
