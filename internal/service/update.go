package service

import (
	"context"
	"errors"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/platform"
	"github.com/ALFO111111/agent-sync/internal/update"
)

// 检查更新：GUI 启动后自动查一次、之后每天一次；设置页"关于"里也能手动查、一键更新。

// UpdateSettings 自动检查开没开、用户跳过了哪个版本。没初始化（向导里）按默认：开着、没跳过。
type UpdateSettings struct {
	Auto    bool   `json:"auto"`
	Skipped string `json:"skipped"`
}

func GetUpdateSettings() UpdateSettings {
	cfg, err := config.Load()
	if err != nil {
		return UpdateSettings{Auto: true}
	}
	return UpdateSettings{Auto: !cfg.NoUpdateCheck, Skipped: cfg.SkipVersion}
}

// SetAutoUpdateCheck 打开或关闭自动检查更新。
func SetAutoUpdateCheck(on bool) error {
	return changeConfig(func(cfg *config.Config) { cfg.NoUpdateCheck = !on })
}

// SkipUpdateVersion "以后再说"：这个版本不再提醒。
func SkipUpdateVersion(version string) error {
	return changeConfig(func(cfg *config.Config) { cfg.SkipVersion = version })
}

func changeConfig(fn func(*config.Config)) error {
	cfg, err := config.Load()
	if err != nil {
		return errors.New(i18n.T("请先完成初始化"))
	}
	fn(cfg)
	return config.Save(cfg)
}

// CheckUpdate 问 GitHub 有没有新版本。
func CheckUpdate(ctx context.Context, current string) (*update.Info, error) {
	return update.Check(ctx, current)
}

// InstallUpdate 下载、校验、换掉正在运行的程序，返回新程序的路径（调用方负责重启）。
// beforeApply 在下载完、真正替换之前调（GUI 用它等正在跑的同步结束）。
// macOS 上顺便重启后台进程 daemon，让它也跑新版本（Windows 的定时任务每次都重新启动 exe，不用管）。
func InstallUpdate(ctx context.Context, info *update.Info, progress func(done, total int64), beforeApply func()) (string, error) {
	if info == nil || !info.Available {
		return "", errors.New(i18n.T("已是最新版本"))
	}
	dir, err := update.DownloadDir()
	if err != nil {
		return "", err
	}
	path, err := update.Download(ctx, info, dir, progress)
	if err != nil {
		return "", err
	}
	if beforeApply != nil {
		beforeApply()
	}
	exe, err := update.Apply(path)
	if err != nil {
		return "", err
	}
	if !config.Sandboxed() {
		platform.RestartDaemon()
	}
	return exe, nil
}
