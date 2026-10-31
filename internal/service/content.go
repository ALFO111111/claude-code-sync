package service

import (
	"errors"

	"github.com/ALFO111111/claude-code-sync/internal/ccswitch"
	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

type ContentItem struct {
	Tool      string `json:"tool"`
	Category  string `json:"category"`
	Enabled   bool   `json:"enabled"`
	Pending   bool   `json:"pending"`
	CCOverlap bool   `json:"ccOverlap"`
}

type ContentOverview struct {
	Items         []ContentItem `json:"items"`
	CCSwitch      string        `json:"ccSwitch"`
	Pending       []string      `json:"pending"`
	CCSwitchKnown bool          `json:"ccSwitchKnown"`
}

func LoadContent() (*ContentOverview, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	status := ccswitch.DetectAt(cfg.Home)
	known := status == ccswitch.Enabled || status == ccswitch.Disabled || status == ccswitch.NotInstalled
	if known && cfg.UpdateCCSwitch(status == ccswitch.Enabled) {
		if err := config.Save(cfg); err != nil {
			return nil, err
		}
	}
	enabled := status == ccswitch.Enabled
	out := &ContentOverview{Items: []ContentItem{}, Pending: []string{}, CCSwitch: string(status), CCSwitchKnown: known}
	for _, category := range config.ContentCategories {
		pending := cfg.ContentPending(category, enabled)
		out.Items = append(out.Items, ContentItem{Tool: config.ToolClaude, Category: category,
			Enabled: cfg.ContentEnabled(category) && !pending, Pending: pending, CCOverlap: config.CCSwitchCategories[category]})
		if pending {
			out.Pending = append(out.Pending, category)
		}
	}
	for _, category := range config.CodexContentCategories {
		out.Items = append(out.Items, ContentItem{Tool: config.ToolCodex, Category: category,
			Enabled: cfg.ContentEnabledFor(config.ToolCodex, category)})
	}
	return out, nil
}

func SetContentEnabled(category string, enabled bool) error {
	return SetToolContentEnabled(config.ToolClaude, category, enabled)
}

func SetToolContentEnabled(tool, category string, enabled bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !config.ContentCategoryKnownFor(tool, category) {
		return errors.New(i18n.T("未知的同步内容类别"))
	}
	status := ccswitch.DetectAt(cfg.Home)
	if tool == config.ToolClaude && enabled && cfg.ContentPending(category, status == ccswitch.Enabled) {
		return errors.New(i18n.T("这个类别可能也在由 CC Switch 同步，请先选择由谁负责"))
	}
	cfg.SetContentEnabledFor(tool, category, enabled)
	return config.Save(cfg)
}

// SetContentOwner 明确可能重复的类别由谁同步。选择 CC Switch 后，本工具不再同步这个类别。
func SetContentOwner(category, owner string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !config.CCSwitchCategories[category] {
		return errors.New(i18n.T("这个类别不需要选择 CC Switch 负责人"))
	}
	if cfg.ContentApproved == nil {
		cfg.ContentApproved = map[string]bool{}
	}
	switch owner {
	case "sync":
		cfg.ContentApproved[category] = true
		cfg.SetContentEnabled(category, true)
	case "ccswitch":
		delete(cfg.ContentApproved, category)
		cfg.SetContentEnabled(category, false)
	default:
		return errors.New(i18n.T("未知的同步负责人"))
	}
	return config.Save(cfg)
}
