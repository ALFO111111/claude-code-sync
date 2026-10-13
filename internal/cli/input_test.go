package cli

import (
	"strings"
	"testing"
)

// 非终端环境（Git Bash、管道）下读口令/路径，空格不能丢。
// 旧实现用 fmt.Scanln，"my sync pass" 只剩 "my"，两台设备密钥就对不上了。
func TestReadLineKeepsSpaces(t *testing.T) {
	got := readLine(strings.NewReader("my sync pass\n"))
	if got != "my sync pass" {
		t.Errorf("含空格的输入被截断: %q", got)
	}
}

func TestReadLineTrimsSurroundingSpace(t *testing.T) {
	got := readLine(strings.NewReader("  abc  \r\n"))
	if got != "abc" {
		t.Errorf("首尾空白应去掉: %q", got)
	}
}
