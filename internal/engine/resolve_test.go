package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeProfileConflict(t *testing.T) (*device, *device, Conflict) {
	t.Helper()
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	writeProfileFile(t, a, "plans/conflict.md", "共同\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	writeProfileFile(t, a, "plans/conflict.md", "A\n")
	writeProfileFile(t, b, "plans/conflict.md", "B\n")
	a.sync(t, Options{})
	b.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", b.home)
	list, err := ListConflicts()
	if err != nil || len(list) != 1 {
		t.Fatalf("准备冲突失败：%+v %v", list, err)
	}
	return a, b, list[0]
}

func TestResolveConflictLocalRemoteAndMerged(t *testing.T) {
	for _, choice := range []string{ResolveLocal, ResolveRemote, ResolveMerged} {
		t.Run(choice, func(t *testing.T) {
			a, b, c := makeProfileConflict(t)
			merged := []byte("手工合并\n")
			if err := ResolveConflict(context.Background(), c.ID, choice, merged); err != nil {
				t.Fatal(err)
			}
			if list, _ := ListConflicts(); len(list) != 0 {
				t.Fatalf("解决后应清除冲突：%+v", list)
			}
			want := "A\n"
			if choice == ResolveLocal {
				want = "B\n"
			} else if choice == ResolveMerged {
				want = string(merged)
			}
			if got := readFile(t, filepath.Join(b.cfg.ClaudeDir, "plans/conflict.md")); got != want {
				t.Fatalf("B 解决结果不对：%q", got)
			}
			a.sync(t, Options{})
			if got := readFile(t, filepath.Join(a.cfg.ClaudeDir, "plans/conflict.md")); got != want {
				t.Fatalf("解决结果应同步回 A：%q", got)
			}
		})
	}
}

func TestResolveConflictBothKeepsTwoSyncableFiles(t *testing.T) {
	a, b, c := makeProfileConflict(t)
	if err := ResolveConflict(context.Background(), c.ID, ResolveBoth, nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(b.cfg.ClaudeDir, "plans"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("应留下两份正式文件：%+v %v", entries, err)
	}
	a.sync(t, Options{})
	b.sync(t, Options{})
	for _, d := range []*device{a, b} {
		entries, _ := os.ReadDir(filepath.Join(d.cfg.ClaudeDir, "plans"))
		if len(entries) != 2 {
			t.Fatalf("副本也应继续同步：%+v", entries)
		}
		var contents []string
		for _, entry := range entries {
			contents = append(contents, readFile(t, filepath.Join(d.cfg.ClaudeDir, "plans", entry.Name())))
		}
		if !strings.Contains(strings.Join(contents, ""), "A\n") || !strings.Contains(strings.Join(contents, ""), "B\n") {
			t.Fatalf("双方内容都应保留：%q", contents)
		}
	}
}

func TestRestoreDeletedContentItem(t *testing.T) {
	a := newDevice(t, "devA", nil)
	abs := writeProfileFile(t, a, "plans/deleted.md", "恢复我\n")
	a.sync(t, Options{})
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	a.sync(t, Options{})
	t.Setenv("AGENT_SYNC_HOME", a.home)
	if err := RestoreContentItem(context.Background(), "plans", "deleted.md"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, abs); got != "恢复我\n" {
		t.Fatalf("手动恢复失败：%q", got)
	}
}
