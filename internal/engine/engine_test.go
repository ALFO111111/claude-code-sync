package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/state"
	"github.com/ALFO111111/agent-sync/internal/storage"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

// newTestApp 在临时目录里搭一台"设备"（见 testutil.NewDevice）并打开它的引擎。
func newTestApp(t *testing.T, deviceID string) *Engine {
	t.Helper()
	testutil.NewDevice(t, deviceID)
	e, err := New(true)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func addProject(t *testing.T, e *Engine, name string) (root, dir string) {
	return testutil.AddProject(t, e.cfg, name)
}

func writeProjectFile(t *testing.T, e *Engine, dir, rel, content string) string {
	return testutil.WriteProjectFile(t, e.cfg, dir, rel, content)
}

var sessionLine = testutil.SessionLine

func (e *Engine) mustSync(t *testing.T) {
	t.Helper()
	if err := e.Sync(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
}

// theOnlyPID 返回当前唯一的绑定 pid。
func (e *Engine) theOnlyPID(t *testing.T) string {
	t.Helper()
	if len(e.st.Bindings) != 1 {
		t.Fatalf("期望 1 个绑定，实际 %d", len(e.st.Bindings))
	}
	for pid := range e.st.Bindings {
		return pid
	}
	return ""
}

// overwriteRemote 模拟另一台设备：直接把新内容加密覆盖到云端。
func (e *Engine) overwriteRemote(t *testing.T, key, content string) {
	t.Helper()
	enc, err := crypt.Seal(e.key, key, []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.Put(context.Background(), key, enc); err != nil {
		t.Fatal(err)
	}
}

func (e *Engine) readRemote(t *testing.T, key string) string {
	t.Helper()
	data, _, err := e.store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := crypt.Open(e.key, key, data)
	if err != nil {
		t.Fatal(err)
	}
	return string(plain)
}

// 问题#2：本地删了（Claude Code 清理旧会话）并标记 Deleted 后，
// 远端 etag 再变化也不应把文件拉回来。
func TestDeletedStaysDeletedWhenRemoteChanges(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	pid := app.theOnlyPID(t)
	key := FileKey(pid, "s1.jsonl")

	// 本地删除，再同步 → 标记 Deleted
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	app.mustSync(t)
	if !app.st.Files[key].Deleted {
		t.Fatal("本地删除后应标记 Deleted")
	}

	// 另一台设备又动了这个文件（etag 变了）
	app.overwriteRemote(t, key, sessionLine(root, "u2"))
	app.mustSync(t)

	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Errorf("已标记 Deleted 的文件不应被拉回，但它又出现了")
	}
}

// 问题#5：非 jsonl 冲突不看时钟（跨设备的时钟不可信），本机优先，云端那份进备份。
func TestNonJsonlConflictLocalWins(t *testing.T) {
	app := newTestApp(t, "devA")
	_, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "config.json", `{"v":1}`)
	// 项目目录里要有会话文件才能被 discover 认出（cwd）
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(filepath.Join(app.cfg.Home, "proj"), "u1"))
	app.mustSync(t)
	pid := app.theOnlyPID(t)
	key := FileKey(pid, "config.json")

	// 另一台设备推了新版本；本机也改了，而且故意把本机 mtime 调得很早
	// （旧逻辑比较 mtime 会误判远端新，把本机真正的编辑覆盖掉）
	app.overwriteRemote(t, key, `{"v":2}`)
	if err := os.WriteFile(abs, []byte(`{"v":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(abs, past, past)

	app.mustSync(t)

	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"v":3}` {
		t.Errorf("本机的编辑应保留，得到 %q", data)
	}
	if got := app.readRemote(t, key); got != `{"v":3}` {
		t.Errorf("云端应是本机推上去的版本，得到 %q", got)
	}
	// 远端那份（v2）应进了备份
	backupDir := filepath.Join(config.Dir(), "backup")
	found := false
	filepath.WalkDir(backupDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(p, "config.json") {
			if b, _ := os.ReadFile(p); string(b) == `{"v":2}` {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Errorf("被覆盖的云端版本应备份到 %s", backupDir)
	}
}

// 问题#8：--dry 必须只读——不删备份、不改 state.json。
func TestDryRunIsReadOnly(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)

	// 放一个 15 天前的备份目录（pruneBackups 的猎物）
	oldBackup := filepath.Join(config.Dir(), "backup", time.Now().Add(-15*24*time.Hour).Format("20060102-150405"))
	if err := os.MkdirAll(oldBackup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldBackup, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 云端多出一台新设备（fetchDevices 会改 state.Devices 缓存）
	info, _ := json.Marshal(state.DeviceInfo{DeviceID: "devB", Name: "B", Projects: map[string]state.ProjectInfo{}})
	enc, _ := crypt.Seal(app.key, "devices/devB.json", info)
	if _, err := app.store.Put(context.Background(), "devices/devB.json", enc); err != nil {
		t.Fatal(err)
	}

	stateBefore, err := os.ReadFile(state.Path())
	if err != nil {
		t.Fatal(err)
	}

	if err := app.Sync(context.Background(), Options{Dry: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(oldBackup, "keep.txt")); err != nil {
		t.Errorf("dry 运行不应删除备份: %v", err)
	}
	stateAfter, err := os.ReadFile(state.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(stateBefore) != string(stateAfter) {
		t.Errorf("dry 运行不应改写 state.json")
	}
}

// 问题#4：项目目录遍历中途失败（权限不够、文件刚好被占用等）时，
// 不能当成"本地全删了"去标 Deleted / 往回拉，要报错并跳过该项目。
func TestUnreadableProjectDirSkipsBinding(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	pid := app.theOnlyPID(t)
	key := FileKey(pid, "s1.jsonl")

	// 注入遍历失败（"整个目录没了"不算失败，那是下面那个测试的场景）
	app.walkFn = func(string, fs.WalkDirFunc) error { return errors.New("模拟遍历失败") }
	err := app.Sync(context.Background(), Options{})
	if err == nil {
		t.Errorf("遍历失败应让同步报错")
	}
	if app.st.Files[key] != nil && app.st.Files[key].Deleted {
		t.Errorf("遍历失败不应把文件误标 Deleted")
	}
	// 本地文件应原样还在（遍历失败只是跳过，不动磁盘）
	if _, serr := os.Stat(filepath.Join(app.cfg.ProjectsDir(), dir, "s1.jsonl")); serr != nil {
		t.Errorf("遍历失败不应动本地文件: %v", serr)
	}
}

// 本机还没有这个项目的会话目录（典型：新机器刚自动绑定了别的设备的项目，
// 本机从没在里面聊过），不是错误——当作空目录，云端文件正常拉下来。
func TestMissingProjectDirPullsRemote(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)

	// 模拟"刚绑定的新机器"：本地目录不存在，也没有文件级状态
	if err := os.RemoveAll(filepath.Join(app.cfg.ProjectsDir(), dir)); err != nil {
		t.Fatal(err)
	}
	app.st.Files = map[string]*state.FileState{}
	if err := state.Save(app.st); err != nil {
		t.Fatal(err)
	}

	if err := app.Sync(context.Background(), Options{}); err != nil {
		t.Fatalf("目录不存在不应报错: %v", err)
	}
	if _, err := os.Stat(filepath.Join(app.cfg.ProjectsDir(), dir, "s1.jsonl")); err != nil {
		t.Errorf("云端文件应被拉回本机: %v", err)
	}
}

// 问题#6 的同步侧表现：另一台设备在我们写入的瞬间并发覆盖时，
// putVerified 会检测到，push 保留本地、不更新状态，下轮同步自然走合并。
func TestPushDetectsConcurrentOverwrite(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	abs := writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1"))
	app.mustSync(t)
	pid := app.theOnlyPID(t)
	key := FileKey(pid, "s1.jsonl")
	oldETag := app.st.Files[key].ETag

	// 模拟另一台设备：我们每次 Put 这个 key，它立刻用自己的内容盖上去
	otherEnc, err := crypt.Seal(app.key, key, []byte(sessionLine(root, "u9")))
	if err != nil {
		t.Fatal(err)
	}
	app.store = &racingStorage{Storage: app.store, key: key, enc: otherEnc}

	// 本机追加一句 → 走 push → 写完立刻被"另一台设备"覆盖
	f, _ := os.OpenFile(abs, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(sessionLine(root, "u2"))
	f.Close()
	it := &item{key: key, rel: "s1.jsonl", b: app.st.Bindings[pid],
		local: &localFile{abs: abs}, st: app.st.Files[key]}
	if err := app.push(context.Background(), it, nil); err != nil {
		t.Fatalf("并发覆盖应静默留待下轮，得到 %v", err)
	}
	if app.st.Files[key].ETag != oldETag {
		t.Errorf("检测到并发覆盖时不应更新状态 etag")
	}
	if got := app.readRemote(t, key); !strings.Contains(got, "u9") {
		t.Errorf("云端应是另一台设备的内容，得到 %q", got)
	}
}

// racingStorage 在指定 key 的 Put 之后立刻覆盖成另一份内容，模拟并发写冲突。
type racingStorage struct {
	storage.Storage
	key string
	enc []byte
}

func (r *racingStorage) Put(ctx context.Context, key string, data []byte) (string, error) {
	etag, err := r.Storage.Put(ctx, key, data)
	if err == nil && key == r.key {
		r.Storage.Put(ctx, key, r.enc)
	}
	return etag, err
}

// noGetETag 模拟"Get 拿不到 etag"的存储（OSS SDK v2 的 GetObject 实测返回空）。
type noGetETag struct{ storage.Storage }

func (n noGetETag) Get(ctx context.Context, key string) ([]byte, string, error) {
	data, _, err := n.Storage.Get(ctx, key)
	return data, "", err
}

// Get 拿不到 etag 时：下载后状态要落 List 的 etag，不能每轮都重复下载同一批文件
// （真实环境出现过：OSS GetObject 返回空 etag → 状态永远对不上 → 每 15 秒重下 39 个文件的死循环）；
// 上传侧的并发校验改成比对内容后，也不受空 etag 影响。
func TestEmptyGetETagDoesNotLoop(t *testing.T) {
	app := newTestApp(t, "devA")
	root, dir := addProject(t, app, "proj")
	writeProjectFile(t, app, dir, "s1.jsonl", sessionLine(root, "u1")) // 让 discover 认出项目
	writeProjectFile(t, app, dir, "a.txt", "hello")
	app.mustSync(t) // 上传（putVerified 已改成比对内容，不受影响）
	pid := app.theOnlyPID(t)
	key := FileKey(pid, "a.txt")
	if app.st.Files[key].ETag == "" {
		t.Fatal("上传后应记录 etag")
	}

	app.store = noGetETag{app.store} // 从这轮起 Get 没有 etag
	app.overwriteRemote(t, key, "world")
	app.mustSync(t) // 下载，状态应落 List 的 etag
	if app.st.Files[key].ETag == "" {
		t.Fatal("下载后应记录 List 的 etag，不能是空")
	}
	if got := app.st.Files[key].Hash; got == "" {
		t.Fatal("下载后应记录 hash")
	}

	app.mustSync(t) // 什么都没有变，不应再下载
	if app.stats.Pulled != 0 || app.stats.Pushed != 0 {
		t.Fatalf("状态对上后不应再传文件，得到 %+v", app.stats)
	}
}

// 同步完成后 state 里要记下时间和统计（GUI 首页"上次同步"用）。
func TestSyncRecordsLastRun(t *testing.T) {
	e := newTestApp(t, "devA")
	root, dir := addProject(t, e, "proj")
	writeProjectFile(t, e, dir, "s1.jsonl", sessionLine(root, "u1"))
	e.mustSync(t)
	if e.st.LastSync.IsZero() {
		t.Error("同步后应记录 LastSync")
	}
	if e.st.LastStats.Pushed != 1 {
		t.Errorf("应记录上传 1 个文件，得到 %+v", e.st.LastStats)
	}
	// 重新加载也还在（持久化）
	st, err := state.Load()
	if err != nil || st.LastSync.IsZero() {
		t.Errorf("LastSync 应持久化到 state.json: %v", err)
	}
}
