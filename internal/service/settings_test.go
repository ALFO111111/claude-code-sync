package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
)

// 设置：Secret 不回传给界面；留空保存 = 沿用原值；存储变了先测连接，连不上不落盘。
func TestSettingsKeepSecretAndValidate(t *testing.T) {
	ctx := context.Background()
	p, root := testInitParams(t, "pass")
	p.Storage.AccessKeySecret = "sk-secret" // local 存储用不到，只验证读写行为
	if _, err := Initialize(ctx, p); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.Storage.AccessKeySecret != "" || !s.HasSecret {
		t.Fatalf("Secret 不应回传给界面，但要标记已设置: %+v", s)
	}

	// 只改设备名和扫描目录：Secret 留空也不能被清掉
	s.DeviceName = "新名字"
	s.ScanRoots = []string{" D:\\code ", ""}
	if err := SaveSettings(ctx, *s); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load()
	if cfg.DeviceName != "新名字" || cfg.Storage.AccessKeySecret != "sk-secret" {
		t.Errorf("保存结果不对: %+v", cfg)
	}
	if len(cfg.ScanRoots) != 1 || cfg.ScanRoots[0] != `D:\code` {
		t.Errorf("扫描目录应去空白、去空行: %q", cfg.ScanRoots)
	}

	// 坏存储配置：拒绝且不落盘
	bad := *s
	bad.Storage.Path = filepath.Join(root, "bad", "\x00invalid")
	if err := SaveSettings(ctx, bad); err == nil {
		t.Error("坏配置应报错")
	}
	if cfg2, _ := config.Load(); cfg2.Storage.Path != cfg.Storage.Path {
		t.Error("坏配置不应落盘")
	}

	// 设备名不能为空
	empty := *s
	empty.DeviceName = "  "
	if err := SaveSettings(ctx, empty); err == nil {
		t.Error("空设备名应报错")
	}
}
