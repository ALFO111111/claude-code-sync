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
	DeviceID    string         `json:"deviceId"`
	DeviceName  string         `json:"deviceName"`
	Home        string         `json:"home,omitempty"`      // 用户目录，默认取系统值
	ClaudeDir   string         `json:"claudeDir,omitempty"` // 默认 <Home>/.claude
	CodexDir    string         `json:"codexDir,omitempty"`  // 默认 $CODEX_HOME 或 <Home>/.codex
	Storage     storage.Config `json:"storage"`
	ScanRoots   []string       `json:"scanRoots,omitempty"`   // 额外的 git 仓库扫描目录
	Ignore      []string       `json:"ignore,omitempty"`      // 不同步的本地目录名或项目 ID
	Language    string         `json:"language,omitempty"`    // 界面语言 zh-CN / en；空 = 跟随系统
	Theme       string         `json:"theme,omitempty"`       // 界面主题 light / dark；空 = 跟随系统
	NoAutostart bool           `json:"noAutostart,omitempty"` // 用户关掉了开机自启（默认开）

	ContentDisabled []string        `json:"contentDisabled,omitempty"` // 用户关闭的内容类别；默认全部开启
	ContentApproved map[string]bool `json:"contentApproved,omitempty"` // CC Switch 自动同步开着，仍明确选择由本工具同步
	CCSwitchKnown   bool            `json:"ccSwitchKnown,omitempty"`   // 是否已有明确的 CC Switch 检测结果
	CCSwitchEnabled bool            `json:"ccSwitchEnabled,omitempty"` // 上次明确的 Auto Sync 状态，用于发现关闭后重新开启
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
	if c.CodexDir == "" {
		if v := os.Getenv("CODEX_HOME"); v != "" {
			c.CodexDir = v
		} else {
			c.CodexDir = filepath.Join(c.Home, ".codex")
		}
	}
	c.Storage.Normalize()
}

// ProjectsDir Claude Code 存会话的目录 ~/.claude/projects。
func (c *Config) ProjectsDir() string { return filepath.Join(c.ClaudeDir, "projects") }

const (
	ToolClaude = "claude"
	ToolCodex  = "codex"
)

const (
	ContentRules    = "rules"
	ContentPlans    = "plans"
	ContentSkills   = "skills"
	ContentAgents   = "agents"
	ContentCommands = "commands"
	ContentTasks    = "tasks"
	ContentSettings = "settings"
	ContentMCP      = "mcp"
)

var ContentCategories = []string{
	ContentRules, ContentPlans, ContentSkills, ContentAgents,
	ContentCommands, ContentTasks, ContentSettings, ContentMCP,
}

// Codex 专有的类别：旧位置的技能 $CODEX_HOME/skills（Codex 仍会读，.system 是它自带的不同步）、会话。
const (
	ContentLegacySkills = "legacy-skills"
	ContentSessions     = "sessions"
)

var CodexContentCategories = []string{ContentRules, ContentSkills, ContentLegacySkills, ContentSessions}

// ContentKey 是内容开关的持久化键。旧版裸类别仍表示 Claude，避免迁移老配置。
func ContentKey(tool, category string) string {
	if tool == "" || tool == ToolClaude {
		return category
	}
	return tool + "/" + category
}

func ContentCategoryKnown(category string) bool {
	return ContentCategoryKnownFor(ToolClaude, category)
}

func ContentCategoryKnownFor(tool, category string) bool {
	categories := ContentCategories
	if tool == ToolCodex {
		categories = CodexContentCategories
	} else if tool != "" && tool != ToolClaude {
		return false
	}
	for _, known := range categories {
		if category == known {
			return true
		}
	}
	return false
}

// CCSwitchCategories 这些类别也可能被 CC Switch 的整库 Auto Sync 带上。
var CCSwitchCategories = map[string]bool{ContentRules: true, ContentSkills: true, ContentMCP: true}

func (c *Config) ContentEnabled(category string) bool {
	return c.ContentEnabledFor(ToolClaude, category)
}

func (c *Config) ContentEnabledFor(tool, category string) bool {
	key := ContentKey(tool, category)
	for _, disabled := range c.ContentDisabled {
		if disabled == key {
			return false
		}
	}
	return true
}

func (c *Config) SetContentEnabled(category string, enabled bool) {
	c.SetContentEnabledFor(ToolClaude, category, enabled)
}

func (c *Config) SetContentEnabledFor(tool, category string, enabled bool) {
	key := ContentKey(tool, category)
	var out []string
	for _, disabled := range c.ContentDisabled {
		if disabled != key {
			out = append(out, disabled)
		}
	}
	if !enabled {
		out = append(out, key)
	}
	c.ContentDisabled = out
}

// UpdateCCSwitch 记录明确检测结果；关闭后重新开启时，旧的“仍由本工具同步”确认失效。
// 返回配置是否发生变化。unknown 不应调用。
func (c *Config) UpdateCCSwitch(enabled bool) bool {
	changed := false
	if enabled && (!c.CCSwitchKnown || !c.CCSwitchEnabled) && len(c.ContentApproved) > 0 {
		c.ContentApproved = map[string]bool{}
		changed = true
	}
	if !c.CCSwitchKnown || c.CCSwitchEnabled != enabled {
		changed = true
	}
	c.CCSwitchKnown, c.CCSwitchEnabled = true, enabled
	return changed
}

func (c *Config) ContentPending(category string, ccSwitchEnabled bool) bool {
	return c.ContentEnabled(category) && ccSwitchEnabled && CCSwitchCategories[category] && !c.ContentApproved[category]
}

// ClaudeStatePath Claude 的全局状态文件：默认在 Home；自定义配置目录/沙箱时跟随 ClaudeDir。
func (c *Config) ClaudeStatePath() string {
	defaultDir := filepath.Join(c.Home, ".claude")
	if fsutil.SamePath(c.ClaudeDir, defaultDir) && os.Getenv("CLAUDE_CONFIG_DIR") == "" {
		return filepath.Join(c.Home, ".claude.json")
	}
	return filepath.Join(c.ClaudeDir, ".claude.json")
}

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

// PrevKeyPath 改口令前的旧密钥。别的设备还没换上新口令时可能传上来旧密钥加密的文件，有它就还能解开。
func PrevKeyPath() string { return filepath.Join(Dir(), "key.prev") }

// LoadPrevKey 读旧密钥；没有（从没改过口令）或损坏返回 nil。
func LoadPrevKey() []byte {
	data, err := os.ReadFile(PrevKeyPath())
	if err != nil {
		return nil
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil
	}
	return key
}

// ReplaceKey 换成新密钥，原来的密钥留作 key.prev。
func ReplaceKey(newKey []byte) error {
	if old, err := LoadKey(); err == nil {
		if err := fsutil.WriteFileAtomic(PrevKeyPath(), []byte(hex.EncodeToString(old))); err != nil {
			return err
		}
	}
	return fsutil.WriteFileAtomic(KeyPath(), []byte(hex.EncodeToString(newKey)))
}
