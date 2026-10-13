//go:build !windows

package i18n

import "os"

// systemLanguage macOS / Linux 按 POSIX 环境变量的优先级取，如 "zh_CN.UTF-8"、"en_US.UTF-8"。
func systemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			return v
		}
	}
	return ""
}
