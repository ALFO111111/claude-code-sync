// Package service 共用业务操作：cli 和 gui 都只是"收集参数 → 调这里 → 展示结果"，
// 不直接碰 engine、config 等底层包。
//
//	init.go        初始化、测试连接
//	status.go      本机概况
//	sync.go        同步、hook 触发的同步
//	projects.go    项目总览（已绑定 + 待认领）
//	settings.go    可在界面修改的设置
//	automation.go  自动同步：Claude Code hooks、定时任务
//	logs.go        日志
//	sessions.go    会话浏览、搜索、在 Claude Code 里继续
//	backups.go     备份的查看、恢复
//	passphrase.go  修改同步口令
//	usage.go       云端用量
//	appearance.go  主题、开机自启
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/engine"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// InitParams 初始化所需的全部参数（CLI 提问 / GUI 向导表单都汇总成它）。
type InitParams struct {
	DeviceName string         `json:"deviceName"`
	Storage    storage.Config `json:"storage"`
	Passphrase string         `json:"passphrase"`
	Language   string         `json:"language"` // 向导里选的界面语言，空 = 跟随系统
}

// InitResult 初始化结果。
type InitResult struct {
	Config   *config.Config `json:"config"`
	Keycheck string         `json:"keycheck"` // "created"（云端首次写入）或 "matched"（与云端已有口令一致）
}

// Initialize 验证参数 → 测云端连通 → 口令校验 → 落盘 config + key。
func Initialize(ctx context.Context, p InitParams) (*InitResult, error) {
	p.DeviceName = strings.TrimSpace(p.DeviceName)
	if p.DeviceName == "" {
		p.DeviceName = DefaultDeviceName()
	}
	if p.DeviceName == "" {
		return nil, errors.New(i18n.T("请填写设备名"))
	}
	if p.Passphrase == "" {
		return nil, errors.New(i18n.T("口令不能为空"))
	}
	if p.Storage.Type == "" {
		return nil, errors.New(i18n.T("存储类型不能为空"))
	}
	p.Storage.Normalize()

	store, err := storage.New(p.Storage)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := store.List(cctx, ""); err != nil {
		return nil, fmt.Errorf(i18n.T("连不上云端，请检查配置和网络: %w"), err)
	}

	key := crypt.DeriveKey(p.Passphrase, crypt.KeySalt)
	keycheck := "matched"
	data, _, err := store.Get(cctx, engine.KeycheckKey)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		enc, err := engine.SealKeycheck(key)
		if err != nil {
			return nil, err
		}
		if _, err := store.Put(cctx, engine.KeycheckKey, enc); err != nil {
			return nil, err
		}
		keycheck = "created"
	case err != nil:
		return nil, err
	default:
		if !engine.CheckKey(key, data) {
			return nil, errors.New(i18n.T("口令与云端不一致！云端已有其他设备用别的口令加密的数据，请输入相同的口令（或清空云端重新开始）"))
		}
	}

	cfg := &config.Config{
		DeviceID:   randomHex(8),
		DeviceName: p.DeviceName,
		Storage:    p.Storage,
		Language:   i18n.Normalize(p.Language),
	}
	cfg.ApplyDefaults()
	if err := config.WriteKey(key); err != nil {
		return nil, err
	}
	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	return &InitResult{Config: cfg, Keycheck: keycheck}, nil
}

// TestStorage 只验证"这套存储配置能不能连上"（向导和设置页的"测试连接"按钮用）。
func TestStorage(ctx context.Context, sc storage.Config) error {
	sc.Normalize()
	store, err := storage.New(sc)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := store.List(cctx, ""); err != nil {
		return fmt.Errorf(i18n.T("连接失败: %w"), err)
	}
	return nil
}

// DefaultDeviceName 默认设备名 = 本机登录用户名（C:\Users\<用户名> 里的那个用户名）。
// 取不到返回空串：界面上不预填，只显示灰色示例提示，让用户自己填。
func DefaultDeviceName() string {
	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if name == "" {
		name = os.Getenv("USERNAME")
	}
	// Windows 上是 "机器名\用户名"，只要用户名
	if i := strings.LastIndexAny(name, `\/`); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSpace(name)
}

// ExePath 本程序的路径（hooks 和定时任务要回调它）。
func ExePath() string {
	p, err := os.Executable()
	if err != nil {
		return config.AppName
	}
	return p
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
