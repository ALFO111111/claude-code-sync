package service

import (
	"errors"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/platform"
)

// 主题和开机自启：设置页上"改了马上生效"的开关（和界面语言一样，不用点保存）。

// 主题。
const (
	ThemeLight = "light"
	ThemeDark  = "dark"
)

// ThemeInfo 界面主题。Setting 是用户的选择（空 = 跟随系统），Dark 是现在实际是不是深色。
type ThemeInfo struct {
	Setting string `json:"setting"`
	Dark    bool   `json:"dark"`
}

// GetTheme 当前主题。没初始化（向导里）就跟随系统。
func GetTheme() ThemeInfo {
	info := ThemeInfo{}
	if cfg, err := config.Load(); err == nil {
		info.Setting = cfg.Theme
	}
	switch info.Setting {
	case ThemeDark:
		info.Dark = true
	case ThemeLight:
	default:
		info.Setting, info.Dark = "", platform.DarkMode()
	}
	return info
}

// SetTheme 切换主题（空 = 跟随系统）并存进配置。
func SetTheme(theme string) error {
	if theme != ThemeLight && theme != ThemeDark {
		theme = ""
	}
	cfg, err := config.Load()
	if err != nil {
		return nil // 向导里还没有配置文件：只在界面上生效
	}
	if cfg.Theme == theme {
		return nil
	}
	cfg.Theme = theme
	return config.Save(cfg)
}

// AutostartInfo 开机自启。Available=false：沙箱模式不碰全机共用的设置。
type AutostartInfo struct {
	Enabled   bool `json:"enabled"`
	Available bool `json:"available"`
}

// GetAutostart 开机自启是否开着（以注册表里实际登记的为准）。
func GetAutostart() AutostartInfo {
	if config.Sandboxed() {
		return AutostartInfo{}
	}
	_, ok := platform.Autostart()
	return AutostartInfo{Enabled: ok, Available: true}
}

// SetAutostart 打开或关闭开机自启，并记进配置（之后 EnsureAutoSync 按它来）。
func SetAutostart(on bool) error {
	if config.Sandboxed() {
		return errors.New(i18n.T("沙箱模式下不能设置开机自启"))
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.NoAutostart = !on
	if err := config.Save(cfg); err != nil {
		return err
	}
	return syncAutostart(cfg)
}
