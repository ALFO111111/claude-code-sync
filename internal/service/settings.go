package service

import (
	"context"
	"errors"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// Settings 可在界面修改的配置。AccessKeySecret 永远不回传给界面（只告诉它"有没有"），
// 保存时留空表示"沿用已保存的"。
type Settings struct {
	DeviceName string         `json:"deviceName"`
	Storage    storage.Config `json:"storage"`
	HasSecret  bool           `json:"hasSecret"`
	ScanRoots  []string       `json:"scanRoots"`
}

func LoadSettings() (*Settings, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s := &Settings{DeviceName: cfg.DeviceName, Storage: cfg.Storage, ScanRoots: cfg.ScanRoots}
	s.HasSecret = s.Storage.AccessKeySecret != ""
	s.Storage.AccessKeySecret = ""
	if s.ScanRoots == nil {
		s.ScanRoots = []string{}
	}
	return s, nil
}

// SaveSettings 存储配置有变化时先测连通，连不上不落盘。
func SaveSettings(ctx context.Context, s Settings) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(s.DeviceName)
	if name == "" {
		return errors.New(i18n.T("设备名不能为空"))
	}
	sc := WithSavedSecret(s.Storage)
	sc.Normalize()
	if sc != cfg.Storage {
		if err := TestStorage(ctx, sc); err != nil {
			return err
		}
	}
	var roots []string
	for _, r := range s.ScanRoots {
		if r = strings.TrimSpace(r); r != "" {
			roots = append(roots, r)
		}
	}
	cfg.DeviceName = name
	cfg.Storage = sc
	cfg.ScanRoots = roots
	return config.Save(cfg)
}

// WithSavedSecret 界面没填 Secret 时补上已保存的（界面拿不到原值，只能留空表示不改）。
func WithSavedSecret(sc storage.Config) storage.Config {
	if sc.AccessKeySecret == "" {
		if cfg, err := config.Load(); err == nil {
			sc.AccessKeySecret = cfg.Storage.AccessKeySecret
		}
	}
	return sc
}
