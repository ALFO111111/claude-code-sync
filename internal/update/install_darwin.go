//go:build darwin

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ALFO111111/agent-sync/internal/i18n"
)

func downloadDir(exe string) string {
	// 换 .app 要能写它所在的目录（一般是 /Applications），探测写权限也探这里
	if b := bundleOf(exe); b != "" {
		return filepath.Dir(b)
	}
	return os.TempDir()
}

// bundleOf exe 所在的 .app（…/X.app/Contents/MacOS/x → …/X.app）；不是从 App 里运行的返回空。
func bundleOf(exe string) string {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
		return ""
	}
	return bundle
}

// apply 挂载 dmg，把里面的 App 换到现在的 App 所在的位置（名字保持用户现在的）。
func apply(exe, dmg string) (string, error) {
	defer os.Remove(dmg)
	bundle := bundleOf(exe)
	if bundle == "" {
		return "", errors.New(i18n.T("不是从 App 里运行的，没法自动更新"))
	}
	mnt, err := os.MkdirTemp("", "ccs-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(mnt)
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-noverify", "-mountpoint", mnt, dmg).CombinedOutput(); err != nil {
		return "", fmt.Errorf(i18n.T("打开安装包失败: %v %s"), err, strings.TrimSpace(string(out)))
	}
	defer func() {
		if exec.Command("hdiutil", "detach", mnt, "-quiet").Run() != nil {
			exec.Command("hdiutil", "detach", mnt, "-force", "-quiet").Run()
		}
	}()
	apps, _ := filepath.Glob(filepath.Join(mnt, "*.app"))
	if len(apps) == 0 {
		return "", errors.New(i18n.T("安装包里没有找到 App"))
	}
	err = replaceBundle(bundle, func(dst string) error {
		// ditto 会保留 App 里的符号链接、权限和签名
		if out, err := exec.Command("ditto", apps[0], dst).CombinedOutput(); err != nil {
			return fmt.Errorf("ditto: %v %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(bundle, "Contents", "MacOS", filepath.Base(exe)), nil
}

func cleanupOld(exe string) {
	if b := bundleOf(exe); b != "" {
		os.RemoveAll(filepath.Join(filepath.Dir(b), "."+filepath.Base(b)+".old"))
		os.RemoveAll(filepath.Join(filepath.Dir(b), "."+filepath.Base(b)+".new"))
	}
}

// detach 新进程自成一组：旧进程退出时不会连带它。
func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
