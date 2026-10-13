package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitEvent(t *testing.T, ch chan struct{}, d time.Duration, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatalf("超时：%s 没有触发", what)
	}
}

func TestWatcherDebounce(t *testing.T) {
	root := t.TempDir()
	ch := make(chan struct{}, 1)
	w := New(root, func() { ch <- struct{}{} })
	w.Debounce = 150 * time.Millisecond
	w.MaxWait = time.Minute
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if err := os.WriteFile(filepath.Join(root, "a.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, 3*time.Second, "写入文件后防抖触发")

	// 停笔期间反复写，只触发一次（去抖把多次合成一次）
	for i := 0; i < 5; i++ {
		f, _ := os.OpenFile(filepath.Join(root, "a.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
		f.WriteString("{}\n")
		f.Close()
		time.Sleep(30 * time.Millisecond)
	}
	waitEvent(t, ch, 3*time.Second, "连续写入后防抖触发")
	select {
	case <-ch:
		t.Fatal("防抖期间不该触发第二次")
	case <-time.After(400 * time.Millisecond):
	}
}

func TestWatcherMaxWait(t *testing.T) {
	root := t.TempDir()
	ch := make(chan struct{}, 1)
	w := New(root, func() { ch <- struct{}{} })
	w.Debounce = time.Hour // 防抖设得极大，能触发说明是 MaxWait 兜底
	w.MaxWait = 300 * time.Millisecond
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// 一直写个不停：防抖永远不满足，应该被 MaxWait 强制触发
	go func() {
		for i := 0; i < 20; i++ {
			f, _ := os.OpenFile(filepath.Join(root, "b.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			f.WriteString("{}\n")
			f.Close()
			time.Sleep(50 * time.Millisecond)
		}
	}()
	waitEvent(t, ch, 3*time.Second, "连续写入到 MaxWait 强制触发")
}

func TestWatcherNewSubdir(t *testing.T) {
	root := t.TempDir()
	ch := make(chan struct{}, 1)
	w := New(root, func() { ch <- struct{}{} })
	w.Debounce = 150 * time.Millisecond
	w.MaxWait = time.Minute
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// 监听开始后才建的子目录，里面写文件也要能触发
	sub := filepath.Join(root, "new-session", "tool-results")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // 等监听器把新目录挂上
	if err := os.WriteFile(filepath.Join(sub, "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, 3*time.Second, "新建子目录里的文件变化")
}

// NewFile 只对目标文件的变化触发；"写临时文件再改名"的保存方式也能捕获。
func TestFileWatcherOnlyTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch := make(chan struct{}, 1)
	w := NewFile(target, func() { ch <- struct{}{} })
	w.Debounce = 150 * time.Millisecond
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// 同目录别的文件变化：不触发
	if err := os.WriteFile(filepath.Join(dir, "other.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
		t.Fatal("别的文件变化不该触发")
	case <-time.After(500 * time.Millisecond):
	}

	// 写临时文件再改名覆盖目标
	tmp := filepath.Join(dir, "settings.json.new")
	if err := os.WriteFile(tmp, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, 3*time.Second, "改名覆盖目标文件后触发")

	// 改名之后继续盯得住
	if err := os.WriteFile(target, []byte(`{"a":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	waitEvent(t, ch, 3*time.Second, "改名后再次写入触发")
}
