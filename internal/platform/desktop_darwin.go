//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

func autostartCommand(exe string) string { return `"` + exe + `" ` + DaemonArg }

func autostart() (string, bool) {
	a, ok := readAgent(daemonLabel)
	if !ok {
		return "", false
	}
	cmd := `"` + a.Args[0] + `"`
	if len(a.Args) > 1 {
		cmd += " " + strings.Join(a.Args[1:], " ")
	}
	return cmd, true
}

func setAutostart(exe string) error {
	return writeAgent(launchAgent{Label: daemonLabel, Args: []string{exe, DaemonArg}, KeepAlive: true})
}

func removeAutostart() error { return removeAgent(daemonLabel) }

func restartDaemon() {
	if _, ok := readAgent(daemonLabel); ok {
		launchctl("kickstart", "-k", guiDomain()+"/"+daemonLabel)
	}
}

func openFolder(path string) error { return exec.Command("open", path).Start() }

// shQuote 给 shell 用的单引号转义：整体用单引号包起来，里面原有的单引号先结束引号、转义、再重新开始。
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// openTerminal 用"终端"App 开一个新窗口，在 dir 里执行 command。
func openTerminal(dir string, command []string) error {
	parts := make([]string, len(command))
	for i, c := range command {
		parts[i] = shQuote(c)
	}
	script := "cd " + shQuote(dir) + " && " + strings.Join(parts, " ")
	as := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(script)
	return exec.Command("osascript",
		"-e", `tell application "Terminal" to do script "`+as+`"`,
		"-e", `tell application "Terminal" to activate`).Start()
}

func darkMode() bool {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	return err == nil && strings.TrimSpace(string(out)) == "Dark"
}
