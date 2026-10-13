package service

import (
	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

// LanguageInfo 界面语言。Setting 是用户的选择（空 = 跟随系统），Effective 是实际生效的语言。
type LanguageInfo struct {
	Setting   string   `json:"setting"`
	Effective string   `json:"effective"`
	Supported []string `json:"supported"`
}

// ApplyLanguage 程序启动时调：按 config.json 的 language 设置语言；没初始化或没设置就跟随系统。
// 之后所有界面文字、命令行输出、错误信息、日志都用这个语言。
func ApplyLanguage() {
	lang := ""
	if cfg, err := config.Load(); err == nil {
		lang = cfg.Language
	}
	i18n.Set(lang)
}

// GetLanguage 当前语言设置。
func GetLanguage() LanguageInfo {
	info := LanguageInfo{Effective: i18n.Lang(), Supported: i18n.Supported}
	if cfg, err := config.Load(); err == nil {
		info.Setting = i18n.Normalize(cfg.Language)
	}
	return info
}

// SetLanguage 立即切换语言（空 = 跟随系统）。已初始化就存进 config.json；
// 还没初始化（向导里切换）只改内存，初始化时由 InitParams.Language 写进去。
func SetLanguage(lang string) error {
	lang = i18n.Normalize(lang)
	i18n.Set(lang)
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	if cfg.Language == lang {
		return nil
	}
	cfg.Language = lang
	return config.Save(cfg)
}
