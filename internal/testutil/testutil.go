// Package testutil 测试辅助：在临时目录里搭一台"设备"。只给 _test.go 用。
//
// 每台设备有独立的数据目录（通过 CLAUDE_CODE_SYNC_HOME 指过去）、独立的用户目录，
// 云端用 local 存储（临时目录）代替 OSS，全程不碰真实数据。
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/project"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// Passphrase 测试设备统一用的口令。
const Passphrase = "test-pass"

// NewDevice 搭一台已初始化的设备，返回它的配置。之后 config.Load() 读到的就是这台。
func NewDevice(t testing.TB, deviceID string) *config.Config {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CODE_SYNC_HOME", filepath.Join(root, "ccs"))
	cfg := &config.Config{
		DeviceID:   deviceID,
		DeviceName: deviceID,
		Home:       home,
		ClaudeDir:  filepath.Join(home, ".claude"), // 显式指定，不受外部 CLAUDE_CONFIG_DIR 影响
		Storage:    storage.Config{Type: "local", Path: filepath.Join(root, "cloud"), Prefix: "v1/"},
	}
	cfg.ApplyDefaults()
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteKey(crypt.DeriveKey(Passphrase, crypt.KeySalt)); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// AddProject 在 ~/.claude/projects 下建一个项目目录，返回项目根路径和编码后的目录名。
func AddProject(t testing.TB, cfg *config.Config, name string) (root, dir string) {
	t.Helper()
	root = filepath.Join(cfg.Home, name)
	dir = project.EncodeDir(root)
	if err := os.MkdirAll(filepath.Join(cfg.ProjectsDir(), dir), 0o700); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

// WriteProjectFile 往项目目录里写一个文件，返回绝对路径。
func WriteProjectFile(t testing.TB, cfg *config.Config, dir, rel, content string) string {
	t.Helper()
	abs := filepath.Join(cfg.ProjectsDir(), dir, filepath.FromSlash(rel))
	if err := fsutil.WriteFileAtomic(abs, []byte(content)); err != nil {
		t.Fatal(err)
	}
	return abs
}

// SessionLine 一行最简的会话记录（带 cwd，discover 靠它认出项目路径）。
func SessionLine(cwd, uuid string) string {
	m, _ := json.Marshal(map[string]string{"uuid": uuid, "sessionId": "s-" + uuid, "cwd": cwd})
	return string(m) + "\n"
}
