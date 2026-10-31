package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/i18n"
)

// WaitPIDEnv 新版本启动时带着旧进程的进程号：等它退出了再打开窗口，免得被单实例锁当成"第二个"直接退出。
const WaitPIDEnv = "AGENT_SYNC_WAIT_PID"

// legacyWaitPIDEnv 改名前的版本（claude-code-sync）更新到新版本时带的是这个名字。
const legacyWaitPIDEnv = "CLAUDE_CODE_SYNC_WAIT_PID"

// ErrNoAsset 这个系统没有安装包（比如 Intel Mac），只能去发布页手动下载。
var ErrNoAsset = i18n.Err("这个系统没有可以自动安装的版本，请到发布页手动下载")

// Download 把安装包下载到 dir，按 SHA256SUMS.txt 校验，返回文件路径。progress 可以为 nil。
func Download(ctx context.Context, info *Info, dir string, progress func(done, total int64)) (string, error) {
	if info.Asset == "" || info.assetURL == "" {
		return "", ErrNoAsset
	}
	if info.sumsURL == "" {
		return "", errors.New(i18n.T("发布里没有 SHA256SUMS.txt，没法校验下载的文件"))
	}
	want, err := expectedSum(ctx, info.sumsURL, info.Asset)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, info.Asset+".download")
	if err := fetch(ctx, info.assetURL, dst, info.size, progress); err != nil {
		os.Remove(dst)
		return "", err
	}
	if got, err := fileSum(dst); err != nil || got != want {
		os.Remove(dst)
		if err == nil {
			err = errors.New(i18n.T("下载的文件校验不通过（可能下载不完整），请重试"))
		}
		return "", err
	}
	return dst, nil
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "agent-sync")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("下载失败: %w"), err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf(i18n.T("下载失败: %s"), resp.Status)
	}
	return resp, nil
}

// expectedSum 从 SHA256SUMS.txt（sha256sum 的输出格式："哈希  文件名"）里找出 name 的哈希。
func expectedSum(ctx context.Context, url, name string) (string, error) {
	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if sum := findSum(string(data), name); sum != "" {
		return sum, nil
	}
	return "", fmt.Errorf(i18n.T("SHA256SUMS.txt 里没有 %s"), name)
}

func findSum(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0])
		}
	}
	return ""
}

func fetch(ctx context.Context, url, dst string, size int64, progress func(done, total int64)) error {
	resp, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.ContentLength > 0 {
		size = resp.ContentLength
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	var done int64
	buf := make([]byte, 256<<10)
	last := time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return err
			}
			done += int64(n)
			if progress != nil && time.Since(last) > 150*time.Millisecond {
				progress(done, size)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return fmt.Errorf(i18n.T("下载失败: %w"), rerr)
		}
	}
	if progress != nil {
		progress(done, size)
	}
	return f.Close()
}

func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DownloadDir 安装包下到哪：Windows 放在 exe 旁边（之后改名换上去，必须同一个盘），macOS 放临时目录（dmg 挂载用完就删）。
// 顺便检查能不能写：没权限就在下载前报错，免得白下。
func DownloadDir() (string, error) {
	exe, err := Executable()
	if err != nil {
		return "", err
	}
	dir := downloadDir(exe)
	probe := filepath.Join(dir, ".agent-sync-update-probe")
	f, err := os.Create(probe)
	if err != nil {
		return "", fmt.Errorf(i18n.T("没有权限写 %s，没法自动更新，请到发布页手动下载"), dir)
	}
	f.Close()
	os.Remove(probe)
	return dir, nil
}

// Executable 正在运行的程序的真实路径。
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return exe, nil
}

// Apply 用下载好的安装包换掉正在运行的程序，返回新程序的路径（Relaunch 用）。
func Apply(downloaded string) (string, error) {
	exe, err := Executable()
	if err != nil {
		return "", err
	}
	return apply(exe, downloaded)
}

// CleanupOld 删掉上次更新留下的旧版本（Windows 运行中的 exe 删不掉，只能下次启动时删）。
func CleanupOld() {
	if exe, err := Executable(); err == nil {
		cleanupOld(exe)
	}
}

// replaceFile 正在运行的 exe 不能覆盖，但能改名：旧的改名成 .old 让位，新的放到原位置；失败就改回去。
func replaceFile(target, src string) error {
	old := target + ".old"
	os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		return err
	}
	if err := os.Rename(src, target); err != nil {
		os.Rename(old, target)
		return err
	}
	return nil
}

// replaceBundle 换掉整个 .app：先完整复制到旁边的临时目录，再两次改名换上去，中途失败原来的 App 还在。
func replaceBundle(bundle string, copyTo func(dst string) error) error {
	parent, base := filepath.Dir(bundle), filepath.Base(bundle)
	tmp := filepath.Join(parent, "."+base+".new")
	old := filepath.Join(parent, "."+base+".old")
	os.RemoveAll(tmp)
	os.RemoveAll(old)
	if err := copyTo(tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(bundle, old); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, bundle); err != nil {
		os.Rename(old, bundle)
		return err
	}
	return os.RemoveAll(old)
}

// Relaunch 启动新版本（不带参数 = 打开窗口），它会等本进程退出后再打开。
func Relaunch(exe string) error {
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), WaitPIDEnv+"="+strconv.Itoa(os.Getpid()))
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// WaitPrevious 新版本启动时调：环境变量里有旧进程号就等它退出（最多 20 秒）。
func WaitPrevious() {
	v := os.Getenv(WaitPIDEnv)
	if v == "" {
		v = os.Getenv(legacyWaitPIDEnv)
	}
	if v == "" {
		return
	}
	os.Unsetenv(WaitPIDEnv) // 别再传给这个进程启动的子进程
	os.Unsetenv(legacyWaitPIDEnv)
	pid, err := strconv.Atoi(v)
	if err != nil {
		return
	}
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline) && !fsutil.ProcessGone(pid); {
		time.Sleep(200 * time.Millisecond)
	}
}
