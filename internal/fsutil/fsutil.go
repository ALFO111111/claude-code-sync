// Package fsutil 文件系统小工具：原子写文件、按 Windows 规则比较路径。
package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

// WriteFileAtomic 先写临时文件再改名，避免写一半被别人读到。
func WriteFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), TempPrefix+"*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// TempPrefix WriteFileAtomic 临时文件的前缀；遍历目录时据此跳过写到一半的文件。
const TempPrefix = ".ccs-tmp-"

// SamePath 不区分大小写、\ 和 / 视为一样、忽略结尾分隔符地比较路径
// （Windows 和 macOS 默认的文件系统都不区分大小写；另一台设备的路径可能是另一种写法）。
func SamePath(a, b string) bool {
	norm := func(p string) string { return CleanPath(strings.ReplaceAll(p, `\`, "/")) }
	return strings.EqualFold(norm(a), norm(b))
}

// CleanPath 规范化路径并去掉结尾分隔符。
func CleanPath(p string) string {
	p = filepath.Clean(filepath.FromSlash(p))
	return strings.TrimRight(p, `\/`)
}
