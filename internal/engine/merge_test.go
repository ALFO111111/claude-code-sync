package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CRLF 文件合并时不应把 \r 带进追加的行。
func TestMissingLinesStripsCR(t *testing.T) {
	local := []byte("{\"uuid\":\"a\"}\r\n")
	remote := []byte("{\"uuid\":\"a\"}\r\n{\"uuid\":\"b\"}\r\n{\"uuid\":\"b\"}\r\n")
	got := missingLines(local, remote, lineKey)
	if len(got) != 1 || string(got[0]) != `{"uuid":"b"}` {
		t.Errorf("应只缺 b 一行（去掉 \r、去重），得到 %q", got)
	}
	text := missingLines([]byte("- [A](a.md) — x\r\n"), []byte("- [A](a.md) — x\r\n- [B](b.md) — y\r\n"), textLineKey)
	if len(text) != 1 || bytes.ContainsRune(text[0], '\r') || !strings.Contains(string(text[0]), "[B](b.md)") {
		t.Errorf("MEMORY.md 应只缺 B 一行，得到 %q", text)
	}
}

// appendLines 只追加：文件里已有的内容（哪怕是我们读完之后别人刚写进去的）原样保留。
func TestAppendLinesKeepsConcurrentWrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	local := []byte(`{"uuid":"a"}` + "\n")
	os.WriteFile(p, local, 0o600)
	// 我们读完之后，Claude Code 又追加了一行
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	f.WriteString(`{"uuid":"c"}` + "\n")
	f.Close()

	merged, err := appendLines(p, local, [][]byte{[]byte(`{"uuid":"b"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if string(merged) != `{"uuid":"a"}`+"\n"+`{"uuid":"b"}`+"\n" {
		t.Errorf("返回值应是 local + 追加的行，得到 %q", merged)
	}
	disk, _ := os.ReadFile(p)
	for _, id := range []string{"a", "b", "c"} {
		if !strings.Contains(string(disk), `"uuid":"`+id+`"`) {
			t.Errorf("磁盘上应保留 %s，得到 %q", id, disk)
		}
	}
	// 最后一行没换行时先补一个
	p2 := filepath.Join(t.TempDir(), "m.md")
	os.WriteFile(p2, []byte("x"), 0o600)
	if got, _ := appendLines(p2, []byte("x"), [][]byte{[]byte("y")}); string(got) != "x\ny\n" {
		t.Errorf("应先补换行，得到 %q", got)
	}
}

// 分叉用的新会话 ID 必须确定：同样的输入永远得到同一个 ID。
// 否则"fork 推上去、主文件推失败"重试时会再造一份内容相同、ID 不同的重复会话。
func TestForkUUIDDeterministic(t *testing.T) {
	a := forkUUID("sess-1", []byte("remote-content"))
	b := forkUUID("sess-1", []byte("remote-content"))
	if a != b {
		t.Errorf("同样输入应得到同样 ID: %q vs %q", a, b)
	}
	if c := forkUUID("sess-1", []byte("other")); a == c {
		t.Errorf("不同输入应得到不同 ID")
	}
	if len(a) != 36 || strings.Count(a, "-") != 4 {
		t.Errorf("应是标准 UUID 格式，得到 %q", a)
	}
}
