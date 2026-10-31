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

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/fsutil"
)

// Binding：一个项目身份证 ↔ 本机的工作目录和 ~/.claude/projects 下的文件夹。
type Binding struct {
	PID     string `json:"pid"`
	Root    string `json:"root"`
	Dir     string `json:"dir"`
	Display string `json:"display,omitempty"`
	// OldRoots 这台电脑上这个项目以前的路径（项目文件夹搬过、改过名）：以前传上去的会话里写的是旧路径，翻译时也要认。
	OldRoots []string `json:"oldRoots,omitempty"`
}

// FileState 记录上次同步时本地文件和云端文件的样子，用来判断哪边变了。
type FileState struct {
	ETag     string `json:"etag,omitempty"`
	Size     int64  `json:"size"`
	Mtime    int64  `json:"mtime"`
	Hash     string `json:"hash,omitempty"`
	BaseHash string `json:"baseHash,omitempty"` // 上次共同版本的内容 hash；正文在本工具 base/ 下
	Deleted  bool   `json:"deleted,omitempty"`  // 本地删了（比如 Claude Code 自动清理），不再拉回来
}

type ProjectInfo struct {
	Root     string   `json:"root"`
	Display  string   `json:"display,omitempty"`
	OldRoots []string `json:"oldRoots,omitempty"` // 以前的路径，见 Binding.OldRoots
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
	Bindings      map[string]*Binding     `json:"bindings"`              // pid → 绑定
	Ignored       map[string]bool         `json:"ignored"`               // 用户选择忽略的 pid
	Files         map[string]*FileState   `json:"files"`                 // 云端 key → 状态
	Devices       map[string]CachedDevice `json:"devices"`               // 其他设备信息缓存
	PublishedHash string                  `json:"publishedHash"`         // 上次上传的本机设备信息
	PublishedAt   time.Time               `json:"publishedAt,omitempty"` // 上次上传设备信息的时间（每小时至少报一次"我还在"）
	LastSync      time.Time               `json:"lastSync,omitempty"`    // 上次同步时间（GUI 首页用）
	LastStats     SyncStats               `json:"lastStats,omitempty"`
	LastChanges   []Change                `json:"lastChanges,omitempty"`  // 上次同步改动了哪些文件（首页"本轮明细"）
	History       *FileState              `json:"history,omitempty"`      // ~/.claude/history.jsonl 上次同步时的样子
	Forks         map[string]string       `json:"forks,omitempty"`        // 分叉出来的会话 ID → 原会话 ID（会话页标记用）
	KeycheckETag  string                  `json:"keycheckEtag,omitempty"` // 上次验证通过的云端口令校验文件
	KeyMismatch   bool                    `json:"keyMismatch,omitempty"`  // 云端口令已在其他设备上修改，本机要输入新口令
	// CodexForks 两台设备各自续写了同一个 Codex 会话时，对方那份另存成的本机会话：
	// "<对方设备ID>/<会话相对路径>" → 本机分叉文件的相对路径。对方之后再续写，就追加到这份分叉里。
	CodexForks map[string]string `json:"codexForks,omitempty"`
	// Moved 项目搬到新文件夹后留下的旧会话目录（~/.claude/projects 下的目录名，小写）→ 原来的项目路径。
	// 原路径不存在期间不再自动发现它：否则旧目录会被当成新项目，把同一批会话再传一遍。
	Moved map[string]string `json:"moved,omitempty"`
}

// 本轮明细里的变化种类。
const (
	ChangePush     = "push"     // 本机的修改传到了云端
	ChangePull     = "pull"     // 其他设备的修改拉到了本机
	ChangeMerge    = "merge"    // 两边都改了，合并了
	ChangeFork     = "fork"     // 会话在两台设备上各自继续了，另存了一份
	ChangeConflict = "conflict" // 两边都改了，保留了本机的，另一台的版本进了备份
)

// Change 一次同步里一个文件的变化。
type Change struct {
	Kind string `json:"kind"`
	PID  string `json:"pid"`
	Tool string `json:"tool,omitempty"`
	Dir  string `json:"dir"`
	Rel  string `json:"rel"`
}

// MaxChanges 本轮明细最多记多少条（首次同步可能上千个文件，全记没意义）。
const MaxChanges = 200

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
	if st.Forks == nil {
		st.Forks = map[string]string{}
	}
	if st.CodexForks == nil {
		st.CodexForks = map[string]string{}
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
