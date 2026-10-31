//go:build darwin

package update

import "os/exec"

// systemProxy macOS 网络设置里的代理；没开返回空。
func systemProxy() string {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return ""
	}
	return pickMacProxy(string(out))
}
