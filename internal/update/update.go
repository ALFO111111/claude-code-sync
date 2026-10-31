// Package update 检查更新、下载新版本、替换自己。
//
//	Check     问 GitHub 最新的正式版（不含预发布），和当前版本比
//	Download  下载本机的安装包（Windows 是 exe，macOS 是 dmg），按 SHA256SUMS.txt 校验
//	Apply     换掉正在运行的程序：Windows 把旧 exe 改名让位，macOS 换掉整个 .app
//	Relaunch  启动新版本；新进程等旧进程退出后才打开窗口（单实例锁）
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ALFO111111/agent-sync/internal/i18n"
)

// Repo 发布新版本的 GitHub 仓库。
const Repo = "ALFO111111/agent-sync"

// apiBase GitHub API 地址（测试换成本地假服务器）。
var apiBase = "https://api.github.com"

// Info 一次检查的结果。
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"` // 有比当前更新的正式版
	Dev       bool   `json:"dev"`       // 当前是开发版（版本号不是 x.y.z），不提示更新
	Notes     string `json:"notes"`     // 更新说明（取自 CHANGELOG 对应的那一节）
	URL       string `json:"url"`       // 发布页
	Published string `json:"published"` // 发布时间 RFC3339
	Asset     string `json:"asset"`     // 本机要下载的文件名；空 = 这个系统没有安装包，只能去发布页手动下载

	assetURL string
	sumsURL  string
	size     int64
}

type release struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Check 问 GitHub 最新的正式版。releases/latest 本来就不返回预发布和草稿。
func Check(ctx context.Context, current string) (*Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agent-sync/"+current)
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("连不上 GitHub: %w"), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(i18n.T("GitHub 返回 %s"), resp.Status)
	}
	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(r.TagName, "v")
	info := &Info{
		Current: current, Latest: latest, Notes: trimNotes(r.Body), URL: r.HTMLURL, Published: r.PublishedAt,
		Dev: parseVersion(current) == nil,
	}
	info.Available = !info.Dev && Newer(latest, current)
	want := assetName(runtime.GOOS, runtime.GOARCH, latest)
	for _, a := range r.Assets {
		switch a.Name {
		case want:
			info.Asset, info.assetURL, info.size = a.Name, a.URL, a.Size
		case "SHA256SUMS.txt":
			info.sumsURL = a.URL
		}
	}
	return info, nil
}

// assetName 各系统的安装包（和 .github/workflows/release.yml 里的文件名一致）。Intel Mac、Linux 没有。
func assetName(goos, goarch, version string) string {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "agent-sync.exe"
	case goos == "darwin" && goarch == "arm64":
		return "agent-sync-" + version + "-macos-arm64.dmg"
	}
	return ""
}

// trimNotes 去掉 GitHub 自动追加的"What's Changed / Full Changelog"，只留 CHANGELOG 那一节。
func trimNotes(body string) string {
	for _, cut := range []string{"## What's Changed", "**Full Changelog**"} {
		if i := strings.Index(body, cut); i >= 0 {
			body = body[:i]
		}
	}
	return strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
}

type version struct {
	nums [3]int
	pre  string // -beta.1 之类的后缀
}

// parseVersion 解析 x.y.z 或 x.y.z-后缀；不是这种格式（比如 dev）返回 nil。
func parseVersion(s string) *version {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nil
	}
	v := &version{pre: pre}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		v.nums[i] = n
	}
	return v
}

// Newer a 是否比 b 新。按数字比（0.3.10 > 0.3.9）；数字一样时正式版比带后缀的新（0.3.1 > 0.3.1-test）。
// 任何一个解析不了都返回 false。
func Newer(a, b string) bool {
	va, vb := parseVersion(a), parseVersion(b)
	if va == nil || vb == nil {
		return false
	}
	for i := range va.nums {
		if va.nums[i] != vb.nums[i] {
			return va.nums[i] > vb.nums[i]
		}
	}
	return va.pre == "" && vb.pre != ""
}
