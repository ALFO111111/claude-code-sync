package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/project"
)

// Usage 云端占用（按对象存储里的实际大小算：已压缩、已加密）。
type Usage struct {
	Total    int64          `json:"total"`
	Objects  int            `json:"objects"`
	Sessions int64          `json:"sessions"` // files/ 下所有项目文件
	History  int64          `json:"history"`  // 输入历史
	Other    int64          `json:"other"`    // 设备信息、口令校验等
	Projects []ProjectUsage `json:"projects"` // 按项目，大的在前
}

// ProjectUsage 一个项目在云端占多少。
type ProjectUsage struct {
	PID     string `json:"pid"`     // 认不出来（所有设备都不再同步它了）时为空
	Display string `json:"display"` // 项目名
	Bytes   int64  `json:"bytes"`
	Files   int    `json:"files"`
	Bound   bool   `json:"bound"` // 本机在同步它
}

// Usage 列一遍云端文件，统计占用。
func (e *Engine) Usage(ctx context.Context) (*Usage, error) {
	if err := e.FetchDevices(ctx); err != nil {
		return nil, fmt.Errorf(i18n.T("读取云端设备信息失败: %w"), err)
	}
	objs, err := e.store.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf(i18n.T("列出云端文件失败: %w"), err)
	}
	// 项目哈希 → 名字：本机绑定的优先，其次其他设备上报的
	names := map[string]ProjectUsage{}
	for _, d := range e.devices {
		for pid, p := range d.Projects {
			names[project.Hash(pid)] = ProjectUsage{PID: pid, Display: p.Display}
		}
	}
	for pid, b := range e.st.Bindings {
		names[project.Hash(pid)] = ProjectUsage{PID: pid, Display: b.Display, Bound: true}
	}

	u := &Usage{Projects: []ProjectUsage{}}
	byHash := map[string]*ProjectUsage{}
	for _, o := range objs {
		u.Total += o.Size
		u.Objects++
		switch {
		case strings.HasPrefix(o.Key, "files/"):
			u.Sessions += o.Size
			h, _, _ := strings.Cut(strings.TrimPrefix(o.Key, "files/"), "/")
			p := byHash[h]
			if p == nil {
				n := names[h]
				p = &n
				byHash[h] = p
			}
			p.Bytes += o.Size
			p.Files++
		case o.Key == HistoryKey:
			u.History += o.Size
		default:
			u.Other += o.Size
		}
	}
	for _, p := range byHash {
		if p.Display == "" && p.PID != "" {
			p.Display = p.PID
		}
		u.Projects = append(u.Projects, *p)
	}
	sort.Slice(u.Projects, func(i, j int) bool { return u.Projects[i].Bytes > u.Projects[j].Bytes })
	return u, nil
}
