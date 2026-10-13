package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/testutil"
)

// 项目总览：别的设备上报的、本机没绑定的项目列为待认领；忽略后消失。
func TestLoadProjectsClaimable(t *testing.T) {
	a, pid := newDeviceWithSession(t, "devA", "")
	rootA := filepath.Join(a.Home, "proj")

	// 设备 B 指向同一个云端，还没同步过
	b := testutil.NewDevice(t, "devB")
	b.Storage = a.Storage
	if err := config.Save(b); err != nil {
		t.Fatal(err)
	}

	ov, err := LoadProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.RemoteError != "" || len(ov.Bound) != 0 {
		t.Fatalf("B 应无绑定、读云端成功: %+v", ov)
	}
	if len(ov.Devices) != 1 || ov.Devices[0].ID != "devA" || ov.Devices[0].Projects != 1 {
		t.Errorf("应看到设备 A: %+v", ov.Devices)
	}
	if len(ov.Claimable) != 1 || ov.Claimable[0].PID != pid || ov.Claimable[0].Root != rootA || ov.Claimable[0].DeviceID != "devA" {
		t.Fatalf("A 的项目应待认领: %+v", ov.Claimable)
	}

	if _, err := Sync(context.Background(), SyncRequest{Quiet: true, IgnorePID: pid}); err != nil {
		t.Fatal(err)
	}
	ov, _ = LoadProjects(context.Background())
	if len(ov.Claimable) != 0 {
		t.Errorf("忽略后不应再待认领: %+v", ov.Claimable)
	}
}

// 云端读不到时，本机绑定照常返回，错误放 RemoteError（界面不至于整页报错）。
func TestLoadProjectsRemoteError(t *testing.T) {
	cfg, _ := newDeviceWithSession(t, "devA", "")
	// 云端混进一个解不开的设备文件（比如别的口令写的），读设备信息必然失败
	bad := filepath.Join(cfg.Storage.Path, "v1", "devices", "devX.json")
	if err := fsutil.WriteFileAtomic(bad, []byte("garbage")); err != nil {
		t.Fatal(err)
	}

	ov, err := LoadProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Bound) != 1 || ov.RemoteError == "" {
		t.Errorf("应返回本机绑定 + 云端错误: %+v", ov)
	}
}

// 首页数据不连云端：其他设备取上次同步时的缓存；会话页的项目列表只读本地。
// 忽略的项目出现在忽略列表里，名字取其他设备上报的。
func TestStatusDevicesAndIgnored(t *testing.T) {
	a, pid := newDeviceWithSession(t, "devA", "")
	b := testutil.NewDevice(t, "devB")
	b.Storage = a.Storage
	if err := config.Save(b); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(context.Background(), SyncRequest{Quiet: true, IgnorePID: pid}); err != nil {
		t.Fatal(err)
	}

	s := LoadStatus()
	if len(s.Devices) != 1 || s.Devices[0].ID != "devA" || s.Devices[0].Projects != 1 || s.LastSyncAt == 0 || s.Projects != 0 {
		t.Errorf("首页状态不对: %+v", s)
	}
	if bound, err := ListBound(); err != nil || len(bound) != 0 {
		t.Errorf("B 没有绑定: %v %+v", err, bound)
	}
	ov, err := LoadProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Ignored) != 1 || ov.Ignored[0].PID != pid || ov.Ignored[0].Display == "" {
		t.Errorf("忽略列表不对: %+v", ov.Ignored)
	}
}
