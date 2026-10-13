package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// 设备信息交换：每台设备往云端 devices/<ID>.json 上报"我有哪些项目、在哪个路径"，
// 并读取其他设备的上报（自动绑定、待认领列表、路径翻译都靠它）。

func (e *Engine) FetchDevices(ctx context.Context) error {
	objs, err := e.store.List(ctx, "devices/")
	if err != nil {
		return err
	}
	e.devices = map[string]state.DeviceInfo{}
	present := map[string]bool{}
	for _, o := range objs {
		id := strings.TrimSuffix(strings.TrimPrefix(o.Key, "devices/"), ".json")
		if id == e.cfg.DeviceID {
			continue
		}
		present[id] = true
		if c, ok := e.st.Devices[id]; ok && c.ETag == o.ETag {
			e.devices[id] = c.Info
			continue
		}
		data, etag, err := e.store.Get(ctx, o.Key)
		if err != nil {
			return err
		}
		plain, err := crypt.Open(e.key, o.Key, data)
		if err != nil {
			return fmt.Errorf("%s: %w", o.Key, err)
		}
		var info state.DeviceInfo
		if err := json.Unmarshal(plain, &info); err != nil {
			return fmt.Errorf("%s: %w", o.Key, err)
		}
		e.st.Devices[id] = state.CachedDevice{ETag: etag, Info: info}
		e.devices[id] = info
	}
	for id := range e.st.Devices {
		if !present[id] {
			delete(e.st.Devices, id)
		}
	}
	return nil
}

func (e *Engine) publishDevice(ctx context.Context) error {
	info := state.DeviceInfo{DeviceID: e.cfg.DeviceID, Name: e.cfg.DeviceName, Home: e.cfg.Home, Projects: map[string]state.ProjectInfo{}}
	for pid, b := range e.st.Bindings {
		info.Projects[pid] = state.ProjectInfo{Root: b.Root, Display: b.Display}
	}
	data, _ := json.Marshal(info)
	h := crypt.SHA256Hex(data)
	if h == e.st.PublishedHash {
		return nil
	}
	info.Updated = time.Now().UTC()
	data, _ = json.Marshal(info)
	key := "devices/" + e.cfg.DeviceID + ".json"
	enc, err := crypt.Seal(e.key, key, data)
	if err != nil {
		return err
	}
	if _, err := e.store.Put(ctx, key, enc); err != nil {
		return err
	}
	e.st.PublishedHash = h
	return nil
}

type remoteProject struct {
	Device state.DeviceInfo
	Info   state.ProjectInfo
}

// remotePIDs 列出其他设备上报的所有项目。
func (e *Engine) remotePIDs() map[string][]remoteProject {
	out := map[string][]remoteProject{}
	for _, d := range e.devices {
		for pid, p := range d.Projects {
			out[pid] = append(out[pid], remoteProject{d, p})
		}
	}
	return out
}

func (e *Engine) ForeignRoots(pid string) []string {
	var roots []string
	for _, d := range e.devices {
		if p, ok := d.Projects[pid]; ok {
			roots = append(roots, p.Root)
		}
	}
	return roots
}
