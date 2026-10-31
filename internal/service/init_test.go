package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/state"
	"github.com/ALFO111111/agent-sync/internal/storage"
	"github.com/ALFO111111/agent-sync/internal/testutil"
)

// ---------- 本包测试共用的小工具 ----------

// testInitParams 一套指向临时目录的初始化参数（local 存储当云端）。返回临时根目录。
func testInitParams(t *testing.T, pass string) (InitParams, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("AGENT_SYNC_HOME", filepath.Join(root, "ccs"))
	return InitParams{
		DeviceName: "测试机",
		Storage:    storage.Config{Type: "local", Path: filepath.Join(root, "cloud"), Prefix: "v1/"},
		Passphrase: pass,
	}, root
}

// newDeviceWithSession 搭一台设备，放一个带会话的项目并同步一次。返回配置和该项目的 pid。
func newDeviceWithSession(t *testing.T, id, content string) (*config.Config, string) {
	t.Helper()
	cfg := testutil.NewDevice(t, id)
	root, dir := testutil.AddProject(t, cfg, "proj")
	testutil.WriteProjectFile(t, cfg, dir, "a1.jsonl", testutil.SessionLine(root, "u1")+content)
	mustSync(t)
	st, err := state.Load()
	if err != nil || len(st.Bindings) != 1 {
		t.Fatalf("应有 1 个绑定: %v %+v", err, st)
	}
	for pid := range st.Bindings {
		return cfg, pid
	}
	return cfg, ""
}

func mustSync(t *testing.T) {
	t.Helper()
	if _, err := Sync(context.Background(), SyncRequest{Quiet: true}); err != nil {
		t.Fatal(err)
	}
}

// ---------- 初始化 ----------

// 初始化：生成 config + key，云端写入 keycheck。
func TestInitialize(t *testing.T) {
	p, _ := testInitParams(t, "口令 带空格")
	res, err := Initialize(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Keycheck != "created" {
		t.Errorf("首次初始化应创建 keycheck，得到 %q", res.Keycheck)
	}
	if res.Config.DeviceID == "" || res.Config.DeviceName != "测试机" {
		t.Errorf("配置不完整: %+v", res.Config)
	}
	// config.json 和 key 都落地了，且能加载
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DeviceName != "测试机" {
		t.Errorf("config.json 内容不对: %+v", cfg)
	}
	if _, err := config.LoadKey(); err != nil {
		t.Errorf("key 文件应可读: %v", err)
	}
}

// 第二台设备用同一口令 init：keycheck 验证通过。
func TestInitializeSamePassphraseMatches(t *testing.T) {
	p, root := testInitParams(t, "same-pass")
	if _, err := Initialize(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	// 模拟第二台设备：同一个云端（local 存储目录），不同的数据目录
	t.Setenv("AGENT_SYNC_HOME", filepath.Join(root, "ccs-deviceB"))
	p.DeviceName = "另一台"
	res, err := Initialize(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Keycheck != "matched" {
		t.Errorf("同口令应匹配，得到 %q", res.Keycheck)
	}
}

// 口令不一致必须报错（防止污染云端数据）。
func TestInitializeWrongPassphrase(t *testing.T) {
	p, root := testInitParams(t, "right-pass")
	if _, err := Initialize(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_HOME", filepath.Join(root, "ccs-deviceB"))
	p.Passphrase = "wrong-pass"
	_, err := Initialize(context.Background(), p)
	if err == nil || !strings.Contains(err.Error(), "口令") {
		t.Errorf("口令不一致应报口令相关错误，得到 %v", err)
	}
}

func TestInitializeValidation(t *testing.T) {
	p, _ := testInitParams(t, "")
	if _, err := Initialize(context.Background(), p); err == nil {
		t.Error("空口令应报错")
	}
	p.Passphrase = "x"
	p.Storage.Type = "oss" // 缺 bucket/AK/SK
	if _, err := Initialize(context.Background(), p); err == nil {
		t.Error("OSS 配置不完整应报错")
	}
}

// ---------- 状态 ----------

func TestLoadStatus(t *testing.T) {
	t.Setenv("AGENT_SYNC_HOME", t.TempDir())
	if LoadStatus().Initialized {
		t.Error("未初始化时 Initialized 应为 false（向导靠它判断）")
	}
	p, _ := testInitParams(t, "pass")
	if _, err := Initialize(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	s := LoadStatus()
	if !s.Initialized || s.DeviceName != "测试机" || s.DeviceID == "" {
		t.Errorf("状态不对: %+v", s)
	}
}

// 默认设备名是登录用户名（不带"机器名\"前缀），不是机器名。
func TestDefaultDeviceNameIsUserName(t *testing.T) {
	name := DefaultDeviceName()
	if strings.ContainsAny(name, `\/`) {
		t.Errorf("不应带域/机器名前缀: %q", name)
	}
	if u := os.Getenv("USERNAME"); u != "" && !strings.EqualFold(name, u) {
		t.Errorf("应等于登录用户名 %q，得到 %q", u, name)
	}
}
