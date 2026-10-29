package ccswitch

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAt(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		want    Status
	}{
		{name: "配置文件缺失", want: NotInstalled},
		{name: "开关都缺失", content: text(`{}`), want: Disabled},
		{name: "开关都关闭", content: text(`{"webdavSync":{"enabled":false,"autoSync":false},"s3Sync":{"enabled":false,"autoSync":false}}`), want: Disabled},
		{name: "WebDAV 自动同步开启", content: text(`{"webdavSync":{"enabled":true,"autoSync":true},"s3Sync":{"enabled":false,"autoSync":false}}`), want: Enabled},
		{name: "S3 自动同步开启", content: text(`{"webdavSync":{"enabled":false,"autoSync":false},"s3Sync":{"enabled":true,"autoSync":true}}`), want: Enabled},
		{name: "WebDAV 只开 enabled", content: text(`{"webdavSync":{"enabled":true,"autoSync":false}}`), want: Disabled},
		{name: "WebDAV 只开 autoSync", content: text(`{"webdavSync":{"enabled":false,"autoSync":true}}`), want: Disabled},
		{name: "S3 只开 enabled", content: text(`{"s3Sync":{"enabled":true,"autoSync":false}}`), want: Disabled},
		{name: "JSON 损坏", content: text(`{"webdavSync":`), want: Unknown},
		{name: "布尔字段类型错误", content: text(`{"webdavSync":{"enabled":"true","autoSync":true}}`), want: Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".cc-switch", "settings.json")
			if tt.content != nil {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(*tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			var beforeContent []byte
			var beforeInfo os.FileInfo
			if tt.content != nil {
				var err error
				beforeContent, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				beforeInfo, err = os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
			}

			if got := DetectAt(root); got != tt.want {
				t.Fatalf("DetectAt() = %q，想要 %q", got, tt.want)
			}

			if tt.content == nil {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("检测缺失配置后不应创建文件，Stat 错误 = %v", err)
				}
				return
			}
			afterContent, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(afterContent, beforeContent) {
				t.Fatal("检测前后配置文件内容发生变化")
			}
			afterInfo, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
				t.Fatalf("检测前后配置文件 mtime 发生变化：%v -> %v", beforeInfo.ModTime(), afterInfo.ModTime())
			}
		})
	}
}

func TestDetectAtReadError(t *testing.T) {
	root := t.TempDir()
	// 目录占用了 settings.json 的路径，ReadFile 会失败，且不是“文件不存在”。
	if err := os.MkdirAll(filepath.Join(root, ".cc-switch", "settings.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DetectAt(root); got != Unknown {
		t.Fatalf("DetectAt() = %q，想要 %q", got, Unknown)
	}
}

func TestDetectUsesUserHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("USERPROFILE", root) // Windows
	t.Setenv("HOME", root)        // 其他系统，便于测试跨平台运行
	dir := filepath.Join(root, ".cc-switch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"s3Sync":{"enabled":true,"autoSync":true}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Detect(); got != Enabled {
		t.Fatalf("Detect() = %q，想要 %q", got, Enabled)
	}
}

func TestStatusValues(t *testing.T) {
	// 这些字符串是给 engine/service 使用的稳定 API，不应随界面语言改变。
	values := map[Status]string{
		NotInstalled: "notInstalled",
		Disabled:     "disabled",
		Enabled:      "enabled",
		Unknown:      "unknown",
	}
	for status, want := range values {
		if string(status) != want {
			t.Errorf("状态值 = %q，想要 %q", status, want)
		}
	}
}

func text(s string) *string { return &s }
