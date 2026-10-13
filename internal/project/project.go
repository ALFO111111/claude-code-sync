// Package project 项目身份证（pid）：跨设备稳定地认出"同一个项目"。
//
// pid 有三种：
//
//	home                       没打开项目时，工作目录就是用户目录
//	git:<仓库地址>[#子目录]      有 git 远程地址的项目，各设备自动对上
//	local:<设备ID>:<目录名>      普通文件夹，需要在其他设备上手动认领
package project

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/platform"
)

// HomePID 没打开项目（工作目录 = 用户目录）时的 pid。
const HomePID = "home"

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// EncodeDir 和 Claude Code 一样，把路径里非字母数字的字符都换成 '-'。
func EncodeDir(root string) string { return nonAlnum.ReplaceAllString(root, "-") }

// Hash pid 的短哈希，用作云端目录名（pid 里可能有 OSS 不允许的字符）。
func Hash(pid string) string {
	h := sha256.Sum256([]byte(pid))
	return hex.EncodeToString(h[:])[:16]
}

// Kind 返回 pid 的种类：home | git | local。
func Kind(pid string) string {
	switch {
	case pid == HomePID:
		return "home"
	case strings.HasPrefix(pid, "git:"):
		return "git"
	}
	return "local"
}

// NormalizeRemote 把同一个仓库的不同写法统一成 host/owner/repo。
func NormalizeRemote(u string) string {
	u = strings.TrimSpace(u)
	hadScheme := false
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		hadScheme = true
	}
	if at := strings.Index(u, "@"); at >= 0 {
		if slash := strings.Index(u, "/"); slash < 0 || at < slash {
			u = u[at+1:]
		}
	}
	if !hadScheme {
		// scp 写法 host:owner/repo
		if c := strings.Index(u, ":"); c > 1 {
			if slash := strings.Index(u, "/"); slash < 0 || c < slash {
				u = u[:c] + "/" + u[c+1:]
			}
		}
	} else {
		// 带 scheme 的写法可能显式带端口（ssh://git@host:22/...、https://host:443/...），剥掉，
		// 否则同一仓库"带端口 clone"和"不带端口 clone"会算成两个 pid，自动绑定失效。
		host, rest := u, ""
		if i := strings.Index(u, "/"); i >= 0 {
			host, rest = u[:i], u[i:]
		}
		if j := strings.LastIndex(host, ":"); j >= 0 && isDigits(host[j+1:]) {
			host = host[:j]
		}
		u = host + rest
	}
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return strings.ToLower(u)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	platform.HideWindow(cmd)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// gitIdentity 返回 git 项目的 pid 和仓库根目录；不是 git 项目或没有远程地址则 ok=false。
// GitIdentity 用 git 远程地址算出 pid；top 是仓库根目录。不是 git 仓库或没有远程地址时 ok=false。
func GitIdentity(cwd string) (pid, top string, ok bool) {
	if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
		return "", "", false
	}
	top, err := runGit(cwd, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return "", "", false
	}
	url, _ := runGit(cwd, "config", "--get", "remote.origin.url")
	if url == "" {
		remotes, _ := runGit(cwd, "remote")
		if first := strings.Fields(remotes); len(first) > 0 {
			url, _ = runGit(cwd, "config", "--get", "remote."+first[0]+".url")
		}
	}
	if url == "" {
		return "", "", false
	}
	top = filepath.Clean(filepath.FromSlash(top))
	pid = "git:" + NormalizeRemote(url)
	if rel, err := filepath.Rel(top, filepath.Clean(cwd)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		pid += "#" + filepath.ToSlash(rel)
	}
	return pid, top, true
}

// ReadCwd 从会话文件里读出这个目录对应的真实工作目录（目录名是有损编码，没法反推）。
func ReadCwd(dir string) string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	sort.Slice(files, func(i, j int) bool { return modTime(files[i]) > modTime(files[j]) })
	for _, f := range files {
		if cwd := firstCwd(f); cwd != "" {
			return cwd
		}
	}
	return ""
}

func firstCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	for i := 0; i < 200; i++ {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 && strings.Contains(string(line), `"cwd"`) {
			var v struct {
				Cwd string `json:"cwd"`
			}
			if json.Unmarshal(line, &v) == nil && v.Cwd != "" {
				return v.Cwd
			}
		}
		if err != nil {
			break
		}
	}
	return ""
}

func modTime(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}
