// Package config 本工具自己的数据目录（~/.claude-code-sync）、config.json 和密钥文件。
package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

const AppName = "claude-code-sync"

type Config struct {
	DeviceID   string         `json:"deviceId"`
	DeviceName string         `json:"deviceName"`
	Home       string         `json:"home,omitempty"`      // 用户目录，默认取系统值
	ClaudeDir  string         `json:"claudeDir,omitempty"` // 默认 <Home>/.claude
	Storage    storage.Config `json:"storage"`
	ScanRoots  []string       `json:"scanRoots,omitempty"` // 额外的 git 仓库扫描目录
	Ignore     []string       `json:"ignore,omitempty"`    // 不同步的本地目录名或项目 ID
	Language   string         `json:"language,omitempty"`  // 界面语言 zh-CN / en；空 = 跟随系统
}

// Dir 是本工具自己的数据目录，可用环境变量 CLAUDE_CODE_SYNC_HOME 覆盖（测试/沙箱用）。
func Dir() string {
	if v := os.Getenv("CLAUDE_CODE_SYNC_HOME"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, "."+AppName)
}

// Sandboxed 是否在测试/沙箱模式（设置了 CLAUDE_CODE_SYNC_HOME）。
// 沙箱里不安装 hooks 和定时任务：它们是全机生效的，装了会让真实环境去调用测试用的 exe。
func Sandboxed() bool { return os.Getenv("CLAUDE_CODE_SYNC_HOME") != "" }

func Path() string    { return filepath.Join(Dir(), "config.json") }
func KeyPath() string { return filepath.Join(Dir(), "key") }

// ApplyDefaults 补全用户目录、Claude 目录、存储前缀等默认值。
func (c *Config) ApplyDefaults() {
	if c.Home == "" {
		c.Home, _ = os.UserHomeDir()
	}
	if c.ClaudeDir == "" {
		// 和 Claude Code 保持一致：设置了 CLAUDE_CONFIG_DIR 时会话就在那里
		if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
			c.ClaudeDir = v
		} else {
			c.ClaudeDir = filepath.Join(c.Home, ".claude")
		}
	}
	c.Storage.Normalize()
}

// ProjectsDir Claude Code 存会话的目录 ~/.claude/projects。
func (c *Config) ProjectsDir() string { return filepath.Join(c.ClaudeDir, "projects") }

// Ignored 任一名字（本地目录名或项目 ID）在忽略列表里就返回 true。
func (c *Config) Ignored(names ...string) bool {
	for _, n := range names {
		for _, ig := range c.Ignore {
			if strings.EqualFold(ig, n) {
				return true
			}
		}
	}
	return false
}

// ErrNotInitialized 还没跑过初始化（没有 config.json）。
var ErrNotInitialized = i18n.Err("还没有初始化，请先运行: %s init（或双击打开图形界面）", AppName)

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotInitialized
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf(i18n.T("配置文件损坏 %s: %w"), Path(), err)
	}
	c.ApplyDefaults()
	return &c, nil
}

func Save(c *Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(Path(), data)
}

// LoadKey 读本地保存的 32 字节密钥。
func LoadKey() ([]byte, error) {
	data, err := os.ReadFile(KeyPath())
	if err != nil {
		return nil, fmt.Errorf(i18n.T("读取密钥失败，请重新运行 init: %w"), err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, errors.New(i18n.T("密钥文件损坏，请重新运行 init"))
	}
	return key, nil
}

// WriteKey 把派生出的密钥以 hex 形式存到本地。
func WriteKey(key []byte) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(KeyPath(), []byte(hex.EncodeToString(key)), 0o600)
}
