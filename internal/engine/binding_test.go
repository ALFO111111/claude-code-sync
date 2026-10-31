package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/project"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

// 停止同步一个已绑定的项目：绑定和文件状态清掉，本机、云端文件都不动，下一轮也不会被自动绑回去；
// 取消忽略后自动绑回，重新比对时不会把云端文件当成"本地删了"。
func TestIgnoreBoundProject(t *testing.T) {
	a := newDevice(t, "devA", nil)
	root, dir := testutil.AddProject(t, a.cfg, "proj")
	abs := testutil.WriteProjectFile(t, a.cfg, dir, "s1.jsonl", sessionLine(root, "u1"))
	pid := a.sync(t, Options{}).theOnlyPID(t)

	e := a.sync(t, Options{Ignore: pid})
	if len(e.st.Bindings) != 0 || !e.st.Ignored[pid] {
		t.Fatalf("应解绑并记入忽略: %+v %+v", e.st.Bindings, e.st.Ignored)
	}
	for k := range e.st.Files {
		if strings.HasPrefix(k, FileKey(pid, "")) {
			t.Errorf("文件状态应清掉: %s", k)
		}
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("本机会话文件不能被删: %v", err)
	}
	if e.readRemote(t, FileKey(pid, "s1.jsonl")) == "" {
		t.Fatal("云端文件不能被删")
	}
	if e = a.sync(t, Options{}); len(e.st.Bindings) != 0 {
		t.Fatal("忽略后不该被自动绑回去")
	}

	// 忽略期间本机文件没了：取消忽略后应从云端拉回（旧状态已清掉，不会误判成"本地删了"）
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	testutil.WriteProjectFile(t, a.cfg, dir, "s2.jsonl", sessionLine(root, "u2")) // 留个会话让 discover 认得出目录
	e = a.sync(t, Options{Unignore: pid})
	if e.theOnlyPID(t) != pid || e.st.Ignored[pid] {
		t.Fatalf("取消忽略后应自动绑回: %+v", e.st.Bindings)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Errorf("取消忽略后云端的文件应拉回本机: %v", err)
	}
}

// gitRepo 在 dir 建一个只有远程地址的 git 仓库（自动绑定只看远程地址）。
func gitRepo(t *testing.T, dir, remote string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", remote}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// 打开的是仓库里的子文件夹（pid 带 #子目录）：另一台设备上同一个仓库换了位置、换了写法的远程地址，
// 也要自动对上"仓库根 + 同一个子目录"，并把路径翻译成本机的。
func TestAutoBindGitSubdirectory(t *testing.T) {
	a := newDevice(t, "devA", nil)
	b := newDevice(t, "devB", a)
	repoA := filepath.Join(a.cfg.Home, "mono")
	repoB := filepath.Join(b.cfg.Home, "mono-clone")
	gitRepo(t, repoA, "git@github.com:acme/mono.git")
	gitRepo(t, repoB, "https://github.com/acme/mono")
	subA := filepath.Join(repoA, "pkg", "app")
	subB := filepath.Join(repoB, "pkg", "app")
	os.MkdirAll(subA, 0o700)
	os.MkdirAll(subB, 0o700)

	dirA := project.EncodeDir(subA)
	testutil.WriteProjectFile(t, a.cfg, dirA, "s1.jsonl", sessionLine(subA, "u1"))
	pid := a.sync(t, Options{}).theOnlyPID(t)
	if pid != "git:github.com/acme/mono#pkg/app" {
		t.Fatalf("子目录项目的 pid 应带 #pkg/app，得到 %q", pid)
	}

	e := b.sync(t, Options{})
	bd := e.st.Bindings[pid]
	if bd == nil || !sameDir(bd.Root, subB) {
		t.Fatalf("B 应自动绑定到 %s，得到 %+v", subB, e.st.Bindings)
	}
	data, err := os.ReadFile(filepath.Join(b.cfg.ProjectsDir(), bd.Dir, "s1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if cwd := project.ReadCwd(filepath.Join(b.cfg.ProjectsDir(), bd.Dir)); !sameDir(cwd, subB) {
		t.Errorf("会话里的 cwd 应翻译成 B 的路径 %s，得到 %q（%s）", subB, cwd, data)
	}
}

// sameDir 是不是同一个文件夹。不比字符串：同一个文件夹可能有长短两种写法
// （git 给的是长文件名，CI 上的临时目录是 C:\Users\RUNNER~1\... 这种短文件名）。
func sameDir(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}

// 项目文件夹改了名（比如 claude-code-sync → agent-sync，git 远程地址也改了）：认领到新路径后，
// 云端会话拉到新的会话目录、路径按新位置翻译；旧会话目录不会被当成新项目再传一遍；
// 之后新加入的设备也能把旧会话里的旧路径翻译过去；老路径重新出现时不会把搬走的项目抢回去。
func TestClaimMovedProject(t *testing.T) {
	for _, git := range []bool{true, false} {
		t.Run(map[bool]string{true: "git", false: "local"}[git], func(t *testing.T) { testClaimMovedProject(t, git) })
	}
}

func testClaimMovedProject(t *testing.T, git bool) {
	a := newDevice(t, "devA", nil)
	oldRoot, oldDir := testutil.AddProject(t, a.cfg, "old-name")
	if err := os.MkdirAll(oldRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if git {
		gitRepo(t, oldRoot, "git@github.com:me/old-name.git")
	}
	testutil.WriteProjectFile(t, a.cfg, oldDir, "s1.jsonl", sessionLine(oldRoot, "u1"))
	e := a.sync(t, Options{})
	pid := e.theOnlyPID(t)
	cloudBefore := countFiles(t, e)

	newRoot := filepath.Join(a.cfg.Home, "new-name")
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	if git {
		if out, err := exec.Command("git", "-C", newRoot, "remote", "set-url", "origin", "git@github.com:me/new-name.git").CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	e = a.sync(t, Options{Claim: &Claim{PID: pid, Root: newRoot}})
	newDir := project.EncodeDir(newRoot)
	if b := e.st.Bindings[pid]; len(e.st.Bindings) != 1 || b == nil || b.Root != newRoot || b.Dir != newDir {
		t.Fatalf("应只剩一个绑定、指向新路径: %+v", e.st.Bindings)
	}
	if got := project.ReadCwd(filepath.Join(a.cfg.ProjectsDir(), newDir)); got != newRoot {
		t.Fatalf("会话应拉到新目录并翻译成新路径，cwd = %q", got)
	}
	if n := countFiles(t, e); n != cloudBefore {
		t.Fatalf("云端文件数从 %d 变成 %d：旧目录被重复上传了", cloudBefore, n)
	}
	testutil.WriteProjectFile(t, a.cfg, newDir, "s2.jsonl", sessionLine(newRoot, "u2"))
	if e = a.sync(t, Options{}); len(e.st.Bindings) != 1 || countFiles(t, e) != cloudBefore+1 {
		t.Fatalf("再同步一轮只应多传新会话，不应发现旧目录: %+v", e.st.Bindings)
	}

	// 新设备认领：A 搬家前传的会话里是 A 的旧路径，也要翻译成本机路径
	c := newDevice(t, "devC", a)
	rootC := filepath.Join(c.cfg.Home, "proj-c")
	if err := os.MkdirAll(rootC, 0o700); err != nil {
		t.Fatal(err)
	}
	c.sync(t, Options{Claim: &Claim{PID: pid, Root: rootC}})
	data, err := os.ReadFile(filepath.Join(c.cfg.ProjectsDir(), project.EncodeDir(rootC), "s1.jsonl"))
	if err != nil || strings.Contains(string(data), "old-name") || !strings.Contains(string(data), "proj-c") {
		t.Fatalf("新设备拿到的旧会话应翻译成本机路径: %v %s", err, data)
	}

	t.Setenv("AGENT_SYNC_HOME", a.home)
	if err := os.MkdirAll(oldRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	e = a.sync(t, Options{})
	if b := e.st.Bindings[pid]; b == nil || b.Root != newRoot {
		t.Fatalf("老路径重新出现后不应把搬走的项目抢回去: %+v", e.st.Bindings)
	}
}

func countFiles(t *testing.T, e *Engine) int {
	t.Helper()
	objs, err := e.store.List(context.Background(), "files/")
	if err != nil {
		t.Fatal(err)
	}
	return len(objs)
}
