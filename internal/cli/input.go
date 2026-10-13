package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// stdinReader 全局共用一个带缓冲的 reader：
// 每次新建 bufio.Reader 会把管道里后面的行一起读走丢掉（init 要连读好几行）。
var stdinReader = bufio.NewReader(os.Stdin)

// readLine 读一整行并去掉首尾空白。不能用 fmt.Scanln：它遇到空格就截断，
// 在 Git Bash / 管道里（stdin 不是真终端）会把 "my sync pass" 读成 "my"，
// 两台设备口令明明一样却派生出不同密钥。
func readLine(r io.Reader) string {
	s, _ := bufio.NewReader(r).ReadString('\n')
	return strings.TrimSpace(s)
}

// prompt 从终端读一行。
func prompt(label string) string {
	fmt.Print(label)
	return readLine(stdinReader)
}

// promptSecret 读口令，真终端下不回显；非终端（管道/Git Bash）退化为整行读取。
func promptSecret(label string) string {
	fmt.Print(label)
	if term.IsTerminal(int(syscall.Stdin)) {
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return readLine(stdinReader)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
