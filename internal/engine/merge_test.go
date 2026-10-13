package engine

import (
	"bytes"
	"strings"
	"testing"
)

// CRLF 文件合并时不应把 \r 带进结果（和 unionTextLines 行为一致）。
func TestUnionLinesStripsCR(t *testing.T) {
	local := []byte("{\"uuid\":\"a\"}\r\n")
	remote := []byte("{\"uuid\":\"a\"}\r\n{\"uuid\":\"b\"}\r\n")
	merged := unionLines(local, remote)
	if bytes.ContainsRune(merged, '\r') {
		t.Errorf("合并结果不应包含 \\r: %q", merged)
	}
	if !strings.Contains(string(merged), `"uuid":"b"`) {
		t.Errorf("远端独有的行应并入: %q", merged)
	}
}

func TestUnionTextLinesStripsCR(t *testing.T) {
	local := []byte("- [A](a.md) — x\r\n")
	remote := []byte("- [A](a.md) — x\r\n- [B](b.md) — y\r\n")
	merged := unionTextLines(local, remote)
	if bytes.ContainsRune(merged, '\r') {
		t.Errorf("合并结果不应包含 \\r: %q", merged)
	}
	if !strings.Contains(string(merged), "[B](b.md)") {
		t.Errorf("远端独有的行应并入: %q", merged)
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
