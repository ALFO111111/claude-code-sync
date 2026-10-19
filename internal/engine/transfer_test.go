package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/project"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// hookStorage 在下载指定 key 时先执行 onGet（模拟"下载的这一会儿，Claude Code 往本地文件里写了东西"），并记录每个 key 被 Get 了几次。
type hookStorage struct {
	storage.Storage
	mu    sync.Mutex
	key   string
	onGet func()
	gets  map[string]int
}

func (h *hookStorage) Get(ctx context.Context, key string) ([]byte, string, error) {
	h.mu.Lock()
	if h.gets == nil {
		h.gets = map[string]int{}
	}
	h.gets[key]++
	fn := h.onGet
	if key != h.key {
		fn = nil
	}
	h.mu.Unlock()
	if fn != nil {
		fn()
	}
	return h.Storage.Get(ctx, key)
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

// 核心场景：另一台设备接着聊了几句（云端多了 u2），本机下载的这一会儿 Claude Code 又写了 u3。
// 以前是"整份下载、整份替换"，u3 会被盖掉；现在只往末尾追加缺的记录，u3 必须还在，并在下一轮传上去。
func TestPullSessionAppendsWithoutLosingConcurrentWrite(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "s1.jsonl")

	app.overwriteRemote(t, key, sessionLine(root, "u1")+sessionLine(root, "u2"))
	once := sync.Once{}
	app.store = &hookStorage{Storage: app.store, key: key, onGet: func() {
		once.Do(func() { appendFile(t, abs, sessionLine(root, "u3")) })
	}}
	app.mustSync(t)

	data, _ := os.ReadFile(abs)
	for _, id := range []string{"u1", "u2", "u3"} {
		if !strings.Contains(string(data), `"uuid":"`+id+`"`) {
			t.Fatalf("本机会话应包含 %s，得到 %q", id, data)
		}
	}
	if app.stats.Pulled != 1 {
		t.Errorf("应记一次下载，得到 %+v", app.stats)
	}

	app.mustSync(t) // 下一轮：发现本机多了 u3，传上去
	if got := app.readRemote(t, key); !strings.Contains(got, `"uuid":"u3"`) || !strings.Contains(got, `"uuid":"u2"`) {
		t.Errorf("云端应同时有 u2、u3，得到 %q", got)
	}
}

// 会话最后一行还没写完（Claude Code 正在写）时，这一轮不动它：不算失败，也不分叉。
func TestSessionWithPartialLastLineIsSkipped(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "s1.jsonl")

	app.overwriteRemote(t, key, sessionLine(root, "u1")+sessionLine(root, "u2"))
	appendFile(t, abs, `{"uuid":"u3","cwd":`) // 写到一半
	app.mustSync(t)
	if app.stats.Forked != 0 || app.stats.Errors != 0 {
		t.Errorf("半行不应引发分叉或失败，得到 %+v", app.stats)
	}
	data, _ := os.ReadFile(abs)
	if strings.Contains(string(data), `"uuid":"u2"`) {
		t.Errorf("写到一半时不应追加，得到 %q", data)
	}

	appendFile(t, abs, `"x"}`+"\n") // 写完了
	app.mustSync(t)
	data, _ = os.ReadFile(abs)
	if !strings.Contains(string(data), `"uuid":"u2"`) || !strings.Contains(string(data), `"uuid":"u3"`) {
		t.Errorf("写完后应合并成 u1+u2+u3，得到 %q", data)
	}
}

// 普通文件（非会话）要整份替换：替换前发现本机刚被改过，就放弃这一轮（不算失败），本机的修改保住。
func TestPullOtherFileSkipsWhenLocalChangesMeanwhile(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	abs := writeProjectFile(t, app, dir, "notes.md", "v1")
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "notes.md")

	app.overwriteRemote(t, key, "v2-remote")
	once := sync.Once{}
	app.store = &hookStorage{Storage: app.store, key: key, onGet: func() {
		once.Do(func() { os.WriteFile(abs, []byte("v3-local-edit"), 0o600) })
	}}
	app.mustSync(t)
	if app.stats.Errors != 0 {
		t.Errorf("本机刚被修改只是跳过，不算失败: %+v", app.stats)
	}
	if data, _ := os.ReadFile(abs); string(data) != "v3-local-edit" {
		t.Fatalf("本机刚写的内容不能被覆盖，得到 %q", data)
	}

	// 下一轮：两边都变了 → 本机优先，云端那份进备份
	app.mustSync(t)
	if got := app.readRemote(t, key); got != "v3-local-edit" {
		t.Errorf("云端应是本机的版本，得到 %q", got)
	}
	found := false
	list, _ := ListBackups()
	for _, r := range list {
		if r.Kind == BackupRemote && r.Rel == "notes.md" {
			data, _ := ReadBackupFile(&r)
			found = string(data) == "v2-remote"
		}
	}
	if !found {
		t.Errorf("云端版本应进备份，备份列表 %+v", list)
	}
}

// 下载覆盖本机文件前，本机旧版本进备份；从备份恢复后，恢复前的版本也进备份。
func TestPullBacksUpLocalAndRestore(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	abs := writeProjectFile(t, app, dir, "memory/a.md", "old")
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "memory/a.md")

	app.overwriteRemote(t, key, "new")
	app.mustSync(t)
	if data, _ := os.ReadFile(abs); string(data) != "new" {
		t.Fatalf("应下载云端的新版本，得到 %q", data)
	}
	list, err := ListBackups()
	if err != nil || len(list) != 1 || list[0].Kind != BackupLocal || list[0].Rel != "memory/a.md" || list[0].Dir != dir {
		t.Fatalf("应有一条本机旧版本的备份，得到 %+v, %v", list, err)
	}
	if data, _ := ReadBackupFile(&list[0]); string(data) != "old" {
		t.Fatalf("备份内容应是本机旧版本，得到 %q", data)
	}

	if _, err := RestoreBackup(list[0].ID); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(abs); string(data) != "old" {
		t.Fatalf("恢复后本机应是旧版本，得到 %q", data)
	}
	list, _ = ListBackups()
	kinds := map[string]int{}
	for _, r := range list {
		kinds[r.Kind]++
	}
	if kinds[BackupRestore] != 1 {
		t.Errorf("恢复前的版本应进备份，得到 %+v", list)
	}

	app.mustSync(t) // 恢复算本机的修改，传到云端
	if got := app.readRemote(t, key); got != "old" {
		t.Errorf("恢复后应同步到云端，得到 %q", got)
	}
}

// 分叉要记进备份列表和 state.Forks（会话页据此标"分叉"）。
func TestForkIsRecorded(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "11111111-1111-4111-8111-111111111111.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "11111111-1111-4111-8111-111111111111.jsonl")

	app.overwriteRemote(t, key, sessionLine(root, "u1")+sessionLine(root, "remote-only"))
	appendFile(t, abs, sessionLine(root, "local-only"))
	app.mustSync(t)

	if app.stats.Forked != 1 || len(app.st.Forks) != 1 {
		t.Fatalf("应分叉一次并记录，得到 %+v / %v", app.stats, app.st.Forks)
	}
	list, _ := ListBackups()
	if len(list) != 1 || list[0].Kind != BackupFork || list[0].Fork == "" || list[0].Dir != dir {
		t.Fatalf("备份列表应有一条分叉记录，得到 %+v", list)
	}
	var kinds []string
	for _, c := range app.Changes() {
		kinds = append(kinds, c.Kind)
	}
	if len(kinds) == 0 || kinds[0] != state.ChangeFork {
		t.Errorf("本轮明细应把分叉排在最前，得到 %v", kinds)
	}
}

// 上传后的并发校验用 Stat（HEAD）比 etag，不再整份下载回来。
func TestPushVerifiesWithoutDownloading(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	h := &hookStorage{Storage: app.store}
	app.store = h
	app.mustSync(t)
	key := FileKey(app.theOnlyPID(t), "s1.jsonl")
	if app.stats.Pushed != 1 {
		t.Fatalf("应上传 1 个文件，得到 %+v", app.stats)
	}
	if n := h.gets[key]; n != 0 {
		t.Errorf("上传后不应再下载回来校验，实际下载了 %d 次", n)
	}
}

// 修改口令：云端全部文件换新密钥；本机不用重新下载；另一台设备停下来，输入新口令后继续。
func TestChangeKeyAcrossDevices(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	oldKey := crypt.DeriveKey("test-pass", crypt.KeySalt)
	newKey := crypt.DeriveKey("new-pass", crypt.KeySalt)

	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	ea, _ := New(true)
	kc, _ := SealKeycheck(oldKey)
	ea.store.Put(context.Background(), KeycheckKey, kc)

	rootA := a.cfg.Home
	writeHome := func(d *device, uuid string) {
		p := homeSession(d)
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(sessionLine(d.cfg.Home, uuid)), 0o600)
	}
	writeHome(a, "u1")
	a.sync(t, Options{})
	b.sync(t, Options{})

	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	if err := ChangeKey(context.Background(), newKey, newKey, nil); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("原口令不对时应拒绝，得到 %v", err)
	}
	var last [2]int
	if err := ChangeKey(context.Background(), oldKey, newKey, func(d, n int) { last = [2]int{d, n} }); err != nil {
		t.Fatal(err)
	}
	if last[0] != last[1] || last[1] == 0 {
		t.Errorf("进度应走到底，得到 %v", last)
	}
	ea = a.sync(t, Options{})
	if ea.stats.Pulled+ea.stats.Pushed+ea.stats.Merged != 0 {
		t.Errorf("改完口令后本机不应重新传文件，得到 %+v", ea.stats)
	}

	// B 还是旧口令：同步要停下来，并记下 KeyMismatch
	t.Setenv("CLAUDE_CODE_SYNC_HOME", b.home)
	eb, _ := New(true)
	if err := eb.Sync(context.Background(), Options{}); !errors.Is(err, ErrKeyChanged) {
		t.Fatalf("B 应发现口令已改，得到 %v", err)
	}
	if st, _ := state.Load(); !st.KeyMismatch {
		t.Error("B 的状态应记下 KeyMismatch")
	}
	if err := AdoptKey(context.Background(), oldKey); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("B 输错口令应被拒绝，得到 %v", err)
	}
	if err := AdoptKey(context.Background(), newKey); err != nil {
		t.Fatal(err)
	}
	// A 接着聊，B 换上新口令后能正常拿到
	t.Setenv("CLAUDE_CODE_SYNC_HOME", a.home)
	appendFile(t, homeSession(a), sessionLine(rootA, "u2"))
	a.sync(t, Options{})
	eb = b.sync(t, Options{})
	data, _ := os.ReadFile(homeSession(b))
	if !strings.Contains(string(data), `"uuid":"u2"`) {
		t.Errorf("B 换上新口令后应拿到 A 的新内容，得到 %q", data)
	}
}

// homeSession 设备用户目录下（没打开项目时）的会话文件 s1.jsonl。
func homeSession(d *device) string {
	return filepath.Join(d.cfg.ProjectsDir(), project.EncodeDir(d.cfg.Home), "s1.jsonl")
}
