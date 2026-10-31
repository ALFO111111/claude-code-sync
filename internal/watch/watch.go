// Package watch 文件监听器：盯着会话目录，文件"消停"后回调通知同步。
//
// 这是 Dropbox / Syncthing 的标准做法（监听器为主、定时扫描兜底）：
// Claude Code 聊天时会不停往 .jsonl 追加内容，每次写入都同步太浪费，
// 所以等它停笔一小会儿（防抖）再同步；要是一直聊个不停，到 MaxWait 也强制同步一次。
// 同步引擎自己下载写文件也会触发事件——没关系，那轮同步会发现没有新变化，
// 不写任何文件，循环自然终止（最多空转一轮）。
package watch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/logging"
)

const (
	DefaultDebounce = 10 * time.Second // 停笔多久后触发同步
	DefaultMaxWait  = 2 * time.Minute  // 一直写个不停时，最多憋这么久

	rootRetryInterval = 250 * time.Millisecond
)

// Watcher 递归监听 root 目录。onChange 在防抖结束后被调用（在新 goroutine 里）。
type Watcher struct {
	root     string
	onChange func()

	Debounce time.Duration
	MaxWait  time.Duration

	only   string // 非空：只盯 root 下这一个文件（NewFile），不递归
	filter func(path string) bool

	fw     *fsnotify.Watcher
	stopCh chan struct{}
	doneCh chan struct{}
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
		doneCh:   make(chan struct{}),
	}
}

// SetFilter 设置路径过滤器，必须在 Start 前调用。
// path 是目录或文件的完整路径；返回 false 表示忽略。目录被忽略时，整棵子树都不会监听。
func (w *Watcher) SetFilter(filter func(path string) bool) {
	w.filter = filter
}

func (w *Watcher) allow(path string) bool {
	return w.filter == nil || w.filter(path)
}

// Start 开始监听（后台 goroutine）。
// 根目录不存在也会成功返回，后台等待目录出现后自动挂载；Stop 会同时结束等待。
func (w *Watcher) Start() error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	attached, err := w.attach(fw)
	if err != nil {
		fw.Close()
		return err
	}
	w.fw = fw
	go w.loop(attached)
	return nil
}

// attach 尝试挂载整棵目录树；返回 false 表示根目录还没准备好，需要稍后重试。
func (w *Watcher) attach(fw *fsnotify.Watcher) (bool, error) {
	if w.only != "" {
		if err := fw.Add(w.root); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	if _, err := os.Stat(w.root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !w.allow(w.root) {
		return true, nil
	}

	return w.addDirs(fw, w.root)
}

func (w *Watcher) addDirs(fw *fsnotify.Watcher, root string) (bool, error) {
	attached := false
	// 已存在的子目录全部加上监听（会话目录下还有 workflows/、tool-results/ 等）。
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 个别目录进不去不耽误整体
		}
		if !d.IsDir() {
			return nil
		}
		if !w.allow(path) {
			return filepath.SkipDir
		}
		if err := fw.Add(path); err == nil {
			attached = true
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return attached, nil
}

// hasWatchedFile 判断延迟挂载前是否已经出现了需要上报的文件。
func (w *Watcher) hasWatchedFile() bool {
	if w.only != "" {
		path := filepath.Join(w.root, w.only)
		info, err := os.Stat(path)
		return err == nil && !info.IsDir() && w.allow(path) &&
			!strings.Contains(filepath.Base(path), fsutil.TempPrefix)
	}

	found := false
	_ = filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if !w.allow(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if w.allow(path) && !strings.Contains(filepath.Base(path), fsutil.TempPrefix) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// Stop 停止监听和等待根目录，可重复调用。
func (w *Watcher) Stop() {
	w.once.Do(func() {
		close(w.stopCh)
		if w.fw != nil {
			w.fw.Close()
		}
	})
	if w.fw != nil {
		<-w.doneCh
	}
}

func (w *Watcher) loop(attached bool) {
	defer close(w.doneCh)

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
	noteChange := func() {
		now := time.Now()
		if timerC == nil {
			firstEvent = now
		}
		if now.Sub(firstEvent) >= w.MaxWait {
			fire() // 写个不停的（长对话），到上限就同步
			return
		}
		if timer == nil {
			timer = time.NewTimer(w.Debounce)
		} else {
			timer.Reset(w.Debounce)
		}
		timerC = timer.C
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	var retry *time.Timer
	var retryC <-chan time.Time
	if !attached {
		retry = time.NewTimer(rootRetryInterval)
		retryC = retry.C
	}
	defer func() {
		if retry != nil {
			retry.Stop()
		}
	}()

	for {
		select {
		case <-w.stopCh:
			return
		case <-retryC:
			ok, err := w.attach(w.fw)
			if err == nil && ok {
				retryC = nil
				// 文件可能在目录出现后、挂载完成前就写好了，补记一次变化避免漏掉。
				if w.hasWatchedFile() {
					noteChange()
				}
			} else {
				retry.Reset(rootRetryInterval)
			}
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			// 原子写的临时文件（.ccs-tmp-*）不算变化，等 rename 成正式文件那次事件
			if strings.Contains(filepath.Base(ev.Name), fsutil.TempPrefix) {
				continue
			}
			if !w.allow(ev.Name) {
				continue
			}
			if w.only != "" && !strings.EqualFold(filepath.Base(ev.Name), w.only) {
				continue
			}
			// 新建或改名进来的子目录要递归补监听，不然里面的文件变化就漏了。
			if w.only == "" {
				info, statErr := os.Stat(ev.Name)
				isDir := statErr == nil && info.IsDir()
				if isDir && ev.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
					_, _ = w.addDirs(w.fw, ev.Name)
				}
				// Windows 会把子目录里的写入报成父目录 Write；若该子目录被过滤，
				// 这个父目录事件无法区分来源。目录元数据本身不是文件内容变化，跳过即可。
				if isDir && ev.Op&(fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
			}
			noteChange()
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
