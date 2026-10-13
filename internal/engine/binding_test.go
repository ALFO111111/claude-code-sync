package engine

import (
	"os"
	"strings"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/testutil"
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
