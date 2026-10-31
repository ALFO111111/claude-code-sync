package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3.2", "0.3.1", true},
		{"0.3.10", "0.3.9", true},
		{"1.0.0", "0.99.99", true},
		{"0.3.1", "0.3.1", false},
		{"0.3.1", "0.3.2", false},
		{"0.3.1", "0.3.1-test", true}, // 正式版比同号的测试版新
		{"0.3.1-beta.1", "0.3.1", false},
		{"v0.3.2", "0.3.1", true},
		{"0.3.2", "dev", false}, // 开发版不比
		{"garbage", "0.3.1", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v，想要 %v", c.a, c.b, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("windows", "amd64", "0.4.0"); got != "agent-sync.exe" {
		t.Errorf("windows: %q", got)
	}
	if got := assetName("darwin", "arm64", "0.4.0"); got != "agent-sync-0.4.0-macos-arm64.dmg" {
		t.Errorf("mac: %q", got)
	}
	if got := assetName("darwin", "amd64", "0.4.0"); got != "" {
		t.Errorf("Intel Mac 没有安装包，得到 %q", got)
	}
}

func TestTrimNotes(t *testing.T) {
	body := "### 修复 / Fixed\r\n\r\n- 修了 A\r\n  Fixed A\r\n\r\n## What's Changed\r\n* x by @y\r\n\r\n**Full Changelog**: https://..."
	want := "### 修复 / Fixed\n\n- 修了 A\n  Fixed A"
	if got := trimNotes(body); got != want {
		t.Errorf("得到 %q\n想要 %q", got, want)
	}
}

func TestFindSum(t *testing.T) {
	h := strings.Repeat("a", 64)
	sums := strings.Repeat("b", 64) + "  agent-sync-0.4.0-macos-arm64.dmg\n" + h + " *agent-sync.exe\n"
	if got := findSum(sums, "agent-sync.exe"); got != h {
		t.Errorf("得到 %q", got)
	}
	if got := findSum(sums, "nope"); got != "" {
		t.Errorf("没有的文件应返回空，得到 %q", got)
	}
}

func TestPickProxy(t *testing.T) {
	for in, want := range map[string]string{
		"127.0.0.1:7890":                    "127.0.0.1:7890",
		"http=1.1.1.1:80;https=2.2.2.2:443": "2.2.2.2:443",
		"ftp=3.3.3.3:21;http=1.1.1.1:80":    "1.1.1.1:80",
		"socks=127.0.0.1:1080":              "socks5://127.0.0.1:1080",
		"ftp=3.3.3.3:21":                    "",
	} {
		if got := pickWinProxy(in); got != want {
			t.Errorf("pickWinProxy(%q) = %q，想要 %q", in, got, want)
		}
	}
	mac := "<dictionary> {\n  HTTPEnable : 1\n  HTTPPort : 7890\n  HTTPProxy : 127.0.0.1\n  HTTPSEnable : 0\n}\n"
	if got := pickMacProxy(mac); got != "127.0.0.1:7890" {
		t.Errorf("pickMacProxy = %q", got)
	}
	if got := pickMacProxy("<dictionary> {\n  HTTPEnable : 0\n}\n"); got != "" {
		t.Errorf("没开代理应返回空，得到 %q", got)
	}
	if u, _ := parseProxy("127.0.0.1:7890"); u == nil || u.String() != "http://127.0.0.1:7890" {
		t.Errorf("parseProxy = %v", u)
	}
}

// fakeGitHub 假的 GitHub：releases/latest + 安装包 + SHA256SUMS.txt。sumOverride 非空时 SHA256SUMS 里写这个（模拟校验不通过）。
func fakeGitHub(t *testing.T, latest string, payload []byte, sumOverride string) *httptest.Server {
	t.Helper()
	name := assetName(runtime.GOOS, runtime.GOARCH, latest)
	if name == "" {
		name = "agent-sync.exe" // 没有安装包的系统上也让 Check 走一遍
	}
	sum := sha256.Sum256(payload)
	sumHex := hex.EncodeToString(sum[:])
	if sumOverride != "" {
		sumHex = sumOverride
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + Repo + "/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v%s","body":"- 新功能\n  New feature\n\n**Full Changelog**: x","html_url":"https://example.com/r","published_at":"2026-11-01T00:00:00Z",
				"assets":[{"name":%q,"browser_download_url":"%s/dl/asset","size":%d},{"name":"SHA256SUMS.txt","browser_download_url":"%s/dl/sums","size":1}]}`,
				latest, name, srv.URL, len(payload), srv.URL)
		case "/dl/asset":
			w.Write(payload)
		case "/dl/sums":
			fmt.Fprintf(w, "%s  %s\n", sumHex, name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = old })
	return srv
}

func TestCheckAndDownload(t *testing.T) {
	payload := []byte("new version bytes")
	fakeGitHub(t, "9.9.9", payload, "")
	info, err := Check(context.Background(), "0.3.2")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "9.9.9" || info.Dev || info.Notes != "- 新功能\n  New feature" || info.URL != "https://example.com/r" {
		t.Fatalf("检查结果不对: %+v", info)
	}
	if assetName(runtime.GOOS, runtime.GOARCH, "9.9.9") == "" {
		t.Skip("这个系统没有安装包，不测下载")
	}
	var last int64
	path, err := Download(context.Background(), info, t.TempDir(), func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(payload) || last != int64(len(payload)) {
		t.Fatalf("下载内容 %q，进度 %d", got, last)
	}

	// 当前已经是最新 / 开发版：不提示
	if info, _ := Check(context.Background(), "9.9.9"); info.Available {
		t.Error("同版本不应提示更新")
	}
	if info, _ := Check(context.Background(), "dev"); info.Available || !info.Dev {
		t.Errorf("开发版: %+v", info)
	}
}

func TestDownloadRejectsBadChecksum(t *testing.T) {
	if assetName(runtime.GOOS, runtime.GOARCH, "9.9.9") == "" {
		t.Skip("这个系统没有安装包")
	}
	fakeGitHub(t, "9.9.9", []byte("tampered"), strings.Repeat("0", 64))
	info, err := Check(context.Background(), "0.3.2")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Download(context.Background(), info, dir, nil); err == nil {
		t.Fatal("校验不通过应报错")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("校验不通过的文件应删掉，剩下 %v", left)
	}
}

func TestDownloadWithoutAsset(t *testing.T) {
	if _, err := Download(context.Background(), &Info{}, t.TempDir(), nil); !errors.Is(err, ErrNoAsset) {
		t.Errorf("没有安装包应返回 ErrNoAsset，得到 %v", err)
	}
}

func TestReplaceFile(t *testing.T) {
	dir := t.TempDir()
	target, src := filepath.Join(dir, "app.exe"), filepath.Join(dir, "app.exe.download")
	os.WriteFile(target, []byte("old"), 0o755)
	os.WriteFile(src, []byte("new"), 0o755)
	if err := replaceFile(target, src); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("换完是 %q", got)
	}
	if got, _ := os.ReadFile(target + ".old"); string(got) != "old" {
		t.Errorf("旧的应留在 .old，得到 %q", got)
	}
	// 新文件不见了：换失败，原来的要改回去
	os.WriteFile(target, []byte("cur"), 0o755)
	if err := replaceFile(target, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("应报错")
	}
	if got, _ := os.ReadFile(target); string(got) != "cur" {
		t.Errorf("失败后原文件应还在，得到 %q", got)
	}
}

func TestReplaceBundle(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "X.app")
	os.MkdirAll(filepath.Join(bundle, "Contents"), 0o755)
	os.WriteFile(filepath.Join(bundle, "Contents", "v"), []byte("old"), 0o644)

	// 复制失败：原来的 App 原封不动，也不留临时目录
	if err := replaceBundle(bundle, func(string) error { return errors.New("boom") }); err == nil {
		t.Fatal("应报错")
	}
	if got, _ := os.ReadFile(filepath.Join(bundle, "Contents", "v")); string(got) != "old" {
		t.Fatalf("失败后原 App 应还在，得到 %q", got)
	}

	err := replaceBundle(bundle, func(dst string) error {
		os.MkdirAll(filepath.Join(dst, "Contents"), 0o755)
		return os.WriteFile(filepath.Join(dst, "Contents", "v"), []byte("new"), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(bundle, "Contents", "v")); string(got) != "new" {
		t.Errorf("换完是 %q", got)
	}
	if left, _ := os.ReadDir(dir); len(left) != 1 {
		t.Errorf("只应剩下 X.app，得到 %v", left)
	}
}
