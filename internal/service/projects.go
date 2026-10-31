package service

import (
	"context"
	"sort"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/project"
	"github.com/ALFO111111/agent-sync/internal/state"
)

type BoundProject struct {
	PID     string `json:"pid"`
	Kind    string `json:"kind"` // home | git | local
	Root    string `json:"root"` // 本机路径
	Display string `json:"display"`
}

// ClaimableProject 其他设备有、本机还没绑定也没忽略的项目。
type ClaimableProject struct {
	PID      string `json:"pid"`
	Kind     string `json:"kind"`
	Device   string `json:"device"` // 来自哪台设备（名字）
	DeviceID string `json:"deviceId"`
	Root     string `json:"root"` // 对方设备上的路径
	Display  string `json:"display"`
}

type DeviceSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Projects int    `json:"projects"`
	LastSeen int64  `json:"lastSeen"` // 毫秒时间戳：它最近一次上报的时间（同步时至少每小时报一次）；0 = 不知道
}

func deviceSummary(id string, d state.DeviceInfo) DeviceSummary {
	s := DeviceSummary{ID: id, Name: d.Name, Projects: len(d.Projects)}
	if !d.Updated.IsZero() {
		s.LastSeen = d.Updated.UnixMilli()
	}
	return s
}

// IgnoredProject 用户忽略的项目（可以取消忽略）。
type IgnoredProject struct {
	PID     string `json:"pid"`
	Kind    string `json:"kind"`
	Display string `json:"display"` // 其他设备上报的名字；没有就空
}

type ProjectsOverview struct {
	Bound     []BoundProject     `json:"bound"`
	Claimable []ClaimableProject `json:"claimable"`
	Ignored   []IgnoredProject   `json:"ignored"`
	Devices   []DeviceSummary    `json:"devices"` // 其他设备
	// 读云端失败（断网等）时退回上次同步时缓存的设备信息，错误放这里，不整体报错。
	RemoteError string `json:"remoteError,omitempty"`
}

// ListBound 本机已绑定的项目，只读本地状态、不连云端（会话页用，打开快）。
func ListBound() ([]BoundProject, error) {
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	return boundOf(st), nil
}

func boundOf(st *state.State) []BoundProject {
	out := []BoundProject{}
	for _, b := range st.Bindings {
		out = append(out, BoundProject{PID: b.PID, Kind: project.Kind(b.PID), Root: b.Root, Display: b.Display})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

// LoadProjects 项目总览（CLI status 和 GUI 项目页共用）。会联网读其他设备的最新信息；
// 读不到（断网等）时退回上次同步时缓存的设备信息，错误放在 RemoteError。
func LoadProjects(ctx context.Context) (*ProjectsOverview, error) {
	e, err := engine.New(true)
	if err != nil {
		return nil, err
	}
	if err := e.FetchDevices(ctx); err != nil {
		ov := buildOverview(e.Config(), e.State(), cachedDevices(e.State()))
		ov.RemoteError = err.Error()
		return ov, nil
	}
	return buildOverview(e.Config(), e.State(), e.Devices()), nil
}

// LoadProjectsCached 和 LoadProjects 一样，但只用上次同步时缓存的设备信息，不联网（界面先用它秒开，再联网刷新）。
func LoadProjectsCached() (*ProjectsOverview, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := state.Load()
	if err != nil {
		return nil, err
	}
	return buildOverview(cfg, st, cachedDevices(st)), nil
}

func cachedDevices(st *state.State) map[string]state.DeviceInfo {
	out := map[string]state.DeviceInfo{}
	for id, c := range st.Devices {
		out[id] = c.Info
	}
	return out
}

func buildOverview(cfg *config.Config, st *state.State, devices map[string]state.DeviceInfo) *ProjectsOverview {
	ov := &ProjectsOverview{Bound: boundOf(st), Claimable: []ClaimableProject{}, Devices: []DeviceSummary{}, Ignored: ignoredOf(st, devices)}
	for id, d := range devices {
		ov.Devices = append(ov.Devices, deviceSummary(id, d))
		for pid, p := range d.Projects {
			if st.Bindings[pid] != nil || st.Ignored[pid] || cfg.Ignored(pid) {
				continue
			}
			ov.Claimable = append(ov.Claimable, ClaimableProject{
				PID: pid, Kind: project.Kind(pid), Device: d.Name, DeviceID: id, Root: p.Root, Display: p.Display,
			})
		}
	}
	sort.Slice(ov.Devices, func(i, j int) bool { return ov.Devices[i].Name < ov.Devices[j].Name })
	sort.Slice(ov.Claimable, func(i, j int) bool {
		if ov.Claimable[i].PID != ov.Claimable[j].PID {
			return ov.Claimable[i].PID < ov.Claimable[j].PID
		}
		return ov.Claimable[i].Device < ov.Claimable[j].Device
	})
	return ov
}

// ignoredOf 忽略列表，名字尽量取其他设备上报的（云端读不到时 devices 为空，只显示 ID）。
func ignoredOf(st *state.State, devices map[string]state.DeviceInfo) []IgnoredProject {
	out := []IgnoredProject{}
	for pid, on := range st.Ignored {
		if !on {
			continue
		}
		p := IgnoredProject{PID: pid, Kind: project.Kind(pid)}
		for _, d := range devices {
			if info, ok := d.Projects[pid]; ok && info.Display != "" {
				p.Display = info.Display
				break
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}
