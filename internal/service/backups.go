package service

import (
	"bytes"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/engine"
	"github.com/ALFO111111/agent-sync/internal/state"
)

// 备份页：同步时被替换掉的内容（以及会话分叉）的列表、查看、恢复。备份本身由 engine 在同步时写。

// BackupItem 备份列表里的一条。
type BackupItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // remote | local | restore | fork（见 engine.Backup*）
	Time     int64  `json:"time"` // 毫秒时间戳
	TimeText string `json:"timeText"`
	PID      string `json:"pid"`
	Tool     string `json:"tool,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Category string `json:"category,omitempty"`
	Project  string `json:"project"` // 项目显示名
	Rel      string `json:"rel"`     // 相对项目目录的路径
	Size     int64  `json:"size"`
	Fork     string `json:"fork,omitempty"` // 分叉另存的新会话（相对项目目录）
	Bound    bool   `json:"bound"`          // 这个项目本机还在同步（分叉记录能跳去会话页）
}

// BackupOverview 备份页的全部数据。
type BackupOverview struct {
	Items    []BackupItem `json:"items"`
	Dir      string       `json:"dir"`
	Total    int64        `json:"total"` // 备份文件一共多大
	KeepDays int          `json:"keepDays"`
}

// ListBackups 备份列表，新的在前。
func ListBackups() (*BackupOverview, error) {
	recs, err := engine.ListBackups()
	if err != nil {
		return nil, err
	}
	st, _ := state.Load()
	byDir := map[string]*state.Binding{}
	if st != nil {
		for _, b := range st.Bindings {
			byDir[strings.ToLower(b.Dir)] = b
		}
	}
	ov := &BackupOverview{Items: []BackupItem{}, Dir: engine.BackupDir(), KeepDays: engine.BackupKeepDays}
	for _, r := range recs {
		it := BackupItem{
			ID: r.ID, Kind: r.Kind, Time: r.Time.UnixMilli(), TimeText: fmtTime(r.Time),
			PID: r.PID, Tool: r.Tool, Scope: r.Scope, Category: r.Category, Project: r.Dir, Rel: r.Rel, Size: r.Size, Fork: r.Fork,
		}
		if r.Scope == "global" {
			it.Project = r.Category
		}
		if b := byDir[strings.ToLower(r.Dir)]; b != nil {
			it.PID, it.Bound = b.PID, true
			if it.Project = b.Display; it.Project == "" {
				it.Project = filepath.Base(b.Root)
			}
		}
		ov.Total += r.Size
		ov.Items = append(ov.Items, it)
	}
	return ov, nil
}

// maxPreview 预览最多读多大：再大的文件界面上也看不过来。
const maxPreview = 2 << 20

// BackupContent 一份备份和本机当前版本的内容（界面上并排对比）。
type BackupContent struct {
	Backup        string `json:"backup"`
	Current       string `json:"current"`
	CurrentExists bool   `json:"currentExists"`
	Same          bool   `json:"same"`     // 和本机当前版本一样（恢复也没变化）
	Binary        bool   `json:"binary"`   // 不是文本，不显示内容
	TooLarge      bool   `json:"tooLarge"` // 太大，只显示开头
}

// ReadBackup 读一份备份和对应的本机文件。
func ReadBackup(id string) (*BackupContent, error) {
	r, err := engine.FindBackup(id)
	if err != nil {
		return nil, err
	}
	data, err := engine.ReadBackupFile(r)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	out := &BackupContent{}
	cur, exists, err := engine.ReadBackupCurrent(cfg, r)
	if err != nil {
		return nil, err
	}
	out.CurrentExists = exists
	out.Same = exists && bytes.Equal(cur, data)
	if sniff := truncUTF8(data, 8192); !utf8.Valid(sniff) || bytes.IndexByte(sniff, 0) >= 0 {
		out.Binary = true
		return out, nil
	}
	if len(data) > maxPreview || len(cur) > maxPreview {
		out.TooLarge = true
	}
	out.Backup, out.Current = string(truncUTF8(data, maxPreview)), string(truncUTF8(cur, maxPreview))
	return out, nil
}

// truncUTF8 截到最多 n 字节，不劈开汉字（劈开的话会被当成"不是 UTF-8"）。
func truncUTF8(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	b = b[:n]
	for i := len(b) - 1; i >= 0 && i >= len(b)-utf8.UTFMax; i-- {
		if utf8.RuneStart(b[i]) {
			if !utf8.FullRune(b[i:]) {
				b = b[:i]
			}
			break
		}
	}
	return b
}

// RestoreBackup 用这份备份替换本机文件（替换前的版本也会备份）。调用方之后要跑一轮同步，把它传到其他设备。
func RestoreBackup(id string) error {
	_, err := engine.RestoreBackup(id)
	return err
}
