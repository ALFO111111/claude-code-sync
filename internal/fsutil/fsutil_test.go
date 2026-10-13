package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

// Windows 路径比较：不区分大小写，正反斜杠、结尾分隔符都不影响。
func TestSamePath(t *testing.T) {
	same := [][2]string{
		{`D:\Work\Proj`, `d:\work\proj`},
		{`D:\work\proj\`, `D:\work\proj`},
		{`D:/work/proj`, `D:\work\proj`},
	}
	for _, c := range same {
		if !SamePath(c[0], c[1]) {
			t.Errorf("%q 和 %q 应视为同一路径", c[0], c[1])
		}
	}
	if SamePath(`D:\work\proj`, `D:\work\proj2`) {
		t.Error("不同路径不应相等")
	}
}

// 原子写：自动建目录，写完不留临时文件。
func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", "b", "f.txt")
	if err := WriteFileAtomic(p, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "x" {
		t.Errorf("内容不对: %q", b)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 1 {
		t.Errorf("不应残留临时文件: %v", entries)
	}
}
