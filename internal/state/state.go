// Package state 本机同步状态 state.json（绑定关系、每个云端文件上次同步时的样子、其他设备信息缓存），
// 以及防止多个同步同时跑的文件锁。
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
)

// Binding：一个项目身份证 ↔ 本机的工作目录和 ~/.claude/projects 下的文件夹。
type Binding struct {
	PID     string `json:"pid"`
	Root    string `json:"root"`
	Dir     string `json:"dir"`
	Display string `json:"display,omitempty"`
}

// FileState 记录上次同步时本地文件和云端文件的样子，用来判断哪边变了。
type FileState struct {
	ETag    string `json:"etag,omitempty"`
	Size    int64  `json:"size"`
	Mtime   int64  `json:"mtime"`
	Hash    string `json:"hash,omitempty"`
	Deleted bool   `json:"deleted,omitempty"` // 本地删了（比如 Claude Code 自动清理），不再拉回来
}

type ProjectInfo struct {
	Root    string `json:"root"`
	Display string `json:"display,omitempty"`
}

// DeviceInfo 每台设备上传一份，告诉别的设备"我这里有哪些项目、在哪个路径"。
type DeviceInfo struct {
	DeviceID string                 `json:"deviceId"`
	Name     string                 `json:"name"`
	Home     string                 `json:"home"`
	Projects map[string]ProjectInfo `json:"projects"`
	Updated  time.Time              `json:"updated"`
}

type CachedDevice struct {
	ETag string     `json:"etag"`
	Info DeviceInfo `json:"info"`
}

type State struct {
	Bindings      map[string]*Binding     `json:"bindings"`           // pid → 绑定
	Ignored       map[string]bool         `json:"ignored"`            // 用户选择忽略的 pid
	Files         map[string]*FileState   `json:"files"`              // 云端 key → 状态
	Devices       map[string]CachedDevice `json:"devices"`            // 其他设备信息缓存
	PublishedHash string                  `json:"publishedHash"`      // 上次上传的本机设备信息
	LastSync      time.Time               `json:"lastSync,omitempty"` // 上次同步时间（GUI 首页用）
	LastStats     SyncStats               `json:"lastStats,omitempty"`
	History       *FileState              `json:"history,omitempty"` // ~/.claude/history.jsonl 上次同步时的样子
}

// SyncStats 一次同步的统计。
type SyncStats struct {
	Pushed  int `json:"pushed"`
	Pulled  int `json:"pulled"`
	Merged  int `json:"merged"`
	Forked  int `json:"forked"`
	Errors  int `json:"errors"`
	History int `json:"history"` // 输入历史从其他设备补进来的次数（不算进 Pulled：hook 靠 Pulled 判断"会话有没有更新"）
}

// Path state.json 的位置。
func Path() string { return filepath.Join(config.Dir(), "state.json") }

// Load 读状态；文件不存在返回空状态，文件损坏则备份后从头开始。
func Load() (*State, error) {
	st := &State{}
	data, err := os.ReadFile(Path())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(data, st); err != nil {
			// 状态文件坏了不能让工具彻底瘫痪：备份后从头开始（同步逻辑会按内容比对，不会丢数据）
			os.Rename(Path(), Path()+".corrupt-"+strconv.FormatInt(time.Now().Unix(), 10))
			st = &State{}
		}
	}
	if st.Bindings == nil {
		st.Bindings = map[string]*Binding{}
	}
	if st.Ignored == nil {
		st.Ignored = map[string]bool{}
	}
	if st.Files == nil {
		st.Files = map[string]*FileState{}
	}
	if st.Devices == nil {
		st.Devices = map[string]CachedDevice{}
	}
	return st, nil
}

func Save(st *State) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(Path(), data)
}
