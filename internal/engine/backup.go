package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/claudeconfig"
	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/logging"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// 备份：同步过程中被替换掉的内容先存一份，保留 BackupKeepDays 天（界面"备份"页可以查看、恢复）。
//
//	backup/<本轮时间>/<项目目录>/<相对路径>.remote    两边都改了：保留了本机的，这是另一台设备的版本
//	backup/<本轮时间>/<项目目录>/<相对路径>.local     下载另一台设备的新版本之前，本机的旧版本
//	backup/<本轮时间>/<项目目录>/<相对路径>.restore   从备份恢复之前，本机当时的版本
//	backup/<本轮时间>/index.jsonl                    每条备份（以及会话分叉）一行
//
// 会话（.jsonl）只追加、不覆盖，所以不需要备份；两边各自续聊时会另存一个新会话（分叉），也记进 index。

// BackupKeepDays 备份保留天数。
const BackupKeepDays = 14

// 备份种类。
const (
	BackupRemote  = "remote"  // 冲突时被替换掉的另一台设备的版本
	BackupLocal   = "local"   // 被下载覆盖前本机的版本
	BackupRestore = "restore" // 从备份恢复前本机的版本
	BackupFork    = "fork"    // 会话分叉（没有备份文件，另一份另存为了新会话）
)

const backupIndex = "index.jsonl"
const runLayout = "20060102-150405"

// BackupRecord 一条备份记录。
type BackupRecord struct {
	ID       string    `json:"-"` // 备份文件相对 backup/ 的路径；分叉为 "fork:<本轮时间>/<新会话文件>"
	Kind     string    `json:"kind"`
	Time     time.Time `json:"time"`
	PID      string    `json:"pid,omitempty"`
	Tool     string    `json:"tool,omitempty"`     // 全局文件所属工具；旧记录为空 = Claude
	Scope    string    `json:"scope,omitempty"`    // 空/project=项目文件；global=工具全局文件
	Category string    `json:"category,omitempty"` // 全局内容类别
	Dir      string    `json:"dir"`                // ~/.claude/projects 下的项目目录
	Rel      string    `json:"rel"`                // 相对项目目录的路径（/ 分隔）
	File     string    `json:"file,omitempty"`     // 备份文件，相对 backup/（/ 分隔）
	Size     int64     `json:"size"`
	Fork     string    `json:"fork,omitempty"` // 分叉另存的新会话（相对项目目录）
}

// BackupDir 备份根目录。
func BackupDir() string { return filepath.Join(config.Dir(), "backup") }

// backups 一轮同步（或一次恢复）的备份写入器：同一轮的备份放进同一个时间目录。
type backups struct {
	runTS string
	log   *logging.Logger
	mu    sync.Mutex // 多个文件并行同步，index.jsonl 要串行追加
}

func newBackups(log *logging.Logger) *backups {
	return &backups{runTS: time.Now().Format(runLayout), log: log}
}

// save 把 data 存成备份并记一条索引。失败只写日志：备份失败不能挡住同步本身。
func (b *backups) save(kind, pid, dir, rel string, data []byte) {
	file := path.Join(b.runTS, dir, rel+"."+kind)
	p := filepath.Join(BackupDir(), filepath.FromSlash(file))
	if err := fsutil.WriteFileAtomic(p, data); err != nil {
		b.log.Printf("备份失败 %s: %v", p, err)
		return
	}
	b.record(BackupRecord{Kind: kind, PID: pid, Dir: dir, Rel: rel, File: file, Size: int64(len(data))})
}

func (b *backups) saveGlobal(kind, tool, category, rel string, data []byte) {
	parts := []string{b.runTS, "_global"}
	if tool != "" && tool != config.ToolClaude {
		parts = append(parts, tool)
	}
	parts = append(parts, category, rel+"."+kind)
	file := path.Join(parts...)
	p := filepath.Join(BackupDir(), filepath.FromSlash(file))
	if err := fsutil.WriteFileAtomic(p, data); err != nil {
		b.log.Printf("备份失败 %s: %v", p, err)
		return
	}
	b.record(BackupRecord{Kind: kind, Tool: tool, Scope: "global", Category: category, Rel: rel, File: file, Size: int64(len(data))})
}

// fork 记一条"会话分叉"。
func (b *backups) fork(pid, dir, rel, forkRel string) {
	b.record(BackupRecord{Kind: BackupFork, PID: pid, Dir: dir, Rel: rel, Fork: forkRel})
}

func (b *backups) record(r BackupRecord) {
	r.Time = time.Now()
	line, _ := json.Marshal(r)
	b.mu.Lock()
	defer b.mu.Unlock()
	p := filepath.Join(BackupDir(), b.runTS, backupIndex)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		b.log.Printf("备份失败 %s: %v", p, err)
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		b.log.Printf("备份失败 %s: %v", p, err)
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

// pruneBackups 删掉超过保留期的备份。
func pruneBackups() {
	entries, _ := os.ReadDir(BackupDir())
	for _, ent := range entries {
		t, err := time.ParseInLocation(runLayout, strings.SplitN(ent.Name(), ".", 2)[0], time.Local)
		if err == nil && time.Since(t) > BackupKeepDays*24*time.Hour {
			os.RemoveAll(filepath.Join(BackupDir(), ent.Name()))
		}
	}
}

// ListBackups 全部备份记录，新的在前。旧版本留下的、没有索引的备份文件也列出来（按路径推断）。
func ListBackups() ([]BackupRecord, error) {
	entries, err := os.ReadDir(BackupDir())
	if errors.Is(err, fs.ErrNotExist) {
		return []BackupRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []BackupRecord{}
	for _, ent := range entries {
		runTime, err := time.ParseInLocation(runLayout, ent.Name(), time.Local)
		if !ent.IsDir() || err != nil {
			continue
		}
		out = append(out, readRun(ent.Name(), runTime)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// readRun 读一个时间目录：先读索引，再补上索引里没有的备份文件（旧版本只存文件、不写索引）。
func readRun(run string, runTime time.Time) []BackupRecord {
	var out []BackupRecord
	indexed := map[string]bool{}
	if data, err := os.ReadFile(filepath.Join(BackupDir(), run, backupIndex)); err == nil {
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			var r BackupRecord
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			if r.Scope == "global" && r.Tool == "" {
				r.Tool = config.ToolClaude
			}
			if r.Kind == BackupFork {
				r.ID = "fork:" + run + "/" + r.Dir + "/" + r.Fork
			} else {
				if _, err := os.Stat(filepath.Join(BackupDir(), filepath.FromSlash(r.File))); err != nil {
					continue // 文件被手动删了
				}
				r.ID = r.File
				indexed[r.File] = true
			}
			out = append(out, r)
		}
	}
	root := filepath.Join(BackupDir(), run)
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || p == filepath.Join(root, backupIndex) {
			return nil
		}
		rel, _ := filepath.Rel(BackupDir(), p)
		file := filepath.ToSlash(rel)
		if indexed[file] {
			return nil
		}
		parts := strings.SplitN(file, "/", 3) // <本轮时间>/<项目目录>/<相对路径>.<种类>
		if len(parts) < 3 {
			return nil
		}
		kind := BackupRemote
		relPath := parts[2]
		if i := strings.LastIndexByte(relPath, '.'); i > 0 {
			switch relPath[i+1:] {
			case BackupRemote, BackupLocal, BackupRestore:
				kind, relPath = relPath[i+1:], relPath[:i]
			}
		}
		info, _ := d.Info()
		r := BackupRecord{ID: file, Kind: kind, Time: runTime, Dir: parts[1], Rel: relPath, File: file}
		if parts[1] == "_global" {
			tool := config.ToolClaude
			category, globalRel, ok := strings.Cut(relPath, "/")
			if ok && category == config.ToolCodex {
				tool = category
				category, globalRel, ok = strings.Cut(globalRel, "/")
			}
			if !ok {
				return nil
			}
			r.Scope, r.Tool, r.Category, r.Dir, r.Rel = "global", tool, category, "", globalRel
		}
		if info != nil {
			r.Size = info.Size()
		}
		out = append(out, r)
		return nil
	})
	return out
}

// FindBackup 按 ID 找备份记录。
func FindBackup(id string) (*BackupRecord, error) {
	list, err := ListBackups()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, errors.New(i18n.T("找不到这份备份（可能已超过保留期被清理）"))
}

// ReadBackupFile 读备份文件的内容。
func ReadBackupFile(r *BackupRecord) ([]byte, error) {
	if r.File == "" {
		return nil, errors.New(i18n.T("这条记录没有备份文件"))
	}
	return os.ReadFile(filepath.Join(BackupDir(), filepath.FromSlash(r.File)))
}

// RestoreBackup 用备份替换本机文件。替换前本机当时的版本也先备份一份（种类 restore），恢复本身也能撤销。
// 持有同步锁，免得和正在跑的同步互相覆盖；恢复后的下一轮同步会把它当作本机的修改传到其他设备。
func RestoreBackup(id string) (*BackupRecord, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return nil, err
	}
	defer unlock()

	r, err := FindBackup(id)
	if err != nil {
		return nil, err
	}
	data, err := ReadBackupFile(r)
	if err != nil {
		return nil, err
	}
	target, err := backupTarget(cfg, r)
	if err != nil {
		return nil, err
	}
	if r.Scope == "global" && (r.Category == config.ContentSettings || r.Category == config.ContentMCP) {
		return r, restoreStructuredBackup(cfg, target, r, data)
	}
	cur, err := os.ReadFile(target)
	switch {
	case err == nil && bytes.Equal(cur, data):
		return r, nil // 本来就是这份
	case err == nil:
		if r.Scope == "global" {
			newBackups(logging.New(true)).saveGlobal(BackupRestore, r.Tool, r.Category, r.Rel, cur)
		} else {
			newBackups(logging.New(true)).save(BackupRestore, r.PID, r.Dir, r.Rel, cur)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	if err := fsutil.WriteFileAtomic(target, data); err != nil {
		return nil, err
	}
	logging.New(true).Printf("已从备份恢复 %s/%s", r.Dir+r.Category, r.Rel)
	return r, nil
}

// ReadBackupCurrent 读取备份对应的本机当前内容；结构化配置只返回受管投影，不返回秘密字段。
func ReadBackupCurrent(cfg *config.Config, r *BackupRecord) ([]byte, bool, error) {
	target, err := backupTarget(cfg, r)
	if err != nil {
		return nil, false, err
	}
	cur, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		if r.Scope == "global" && (r.Category == config.ContentSettings || r.Category == config.ContentMCP) {
			cur = []byte("{}")
		} else {
			return nil, false, nil
		}
	} else if err != nil {
		return nil, false, err
	}
	if r.Scope == "global" && r.Category == config.ContentSettings {
		data, err := claudeconfig.SettingsProjection(cur)
		return data, true, err
	}
	if r.Scope == "global" && r.Category == config.ContentMCP {
		st, err := state.Load()
		if err != nil {
			return nil, false, err
		}
		e := &Engine{cfg: cfg, st: st}
		data, _, err := claudeconfig.MCPProjection(cur, e.mcpToCloud)
		return data, true, err
	}
	return cur, true, nil
}

func restoreStructuredBackup(cfg *config.Config, target string, r *BackupRecord, data []byte) error {
	cur, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		cur = []byte("{}")
	} else if err != nil {
		return err
	}
	var current []byte
	var mapper claudeconfig.PathMapper
	if r.Category == config.ContentMCP {
		st, err := state.Load()
		if err != nil {
			return err
		}
		e := &Engine{cfg: cfg, st: st}
		mapper = e.mcpToLocal
		current, _, err = claudeconfig.MCPProjection(cur, e.mcpToCloud)
		if err != nil {
			return err
		}
	} else {
		current, err = claudeconfig.SettingsProjection(cur)
		if err != nil {
			return err
		}
	}
	if bytes.Equal(current, data) {
		return nil
	}
	newBackups(logging.New(true)).saveGlobal(BackupRestore, r.Tool, r.Category, r.Rel, current)
	var next []byte
	if r.Category == config.ContentSettings {
		next, err = claudeconfig.ApplySettings(cur, data)
	} else {
		next, _, err = claudeconfig.ApplyMCP(cur, data, mapper)
	}
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(target, next); err != nil {
		return err
	}
	logging.New(true).Printf("已从备份恢复 %s/%s", r.Category, r.Rel)
	return nil
}

func backupTarget(cfg *config.Config, r *BackupRecord) (string, error) {
	if r.Dir != "" && !filepath.IsLocal(r.Dir) || !filepath.IsLocal(filepath.FromSlash(r.Rel)) {
		return "", errors.New(i18n.T("非法路径"))
	}
	if r.Scope != "global" {
		return filepath.Join(cfg.ProjectsDir(), r.Dir, filepath.FromSlash(r.Rel)), nil
	}
	if r.Tool == "" || r.Tool == config.ToolClaude {
		if r.Category == config.ContentSettings {
			return filepath.Join(cfg.ClaudeDir, "settings.json"), nil
		}
		if r.Category == config.ContentMCP {
			return cfg.ClaudeStatePath(), nil
		}
	}
	if def, ok := findProfileDef(cfg, r.Tool, r.Category); ok && def.allows(r.Rel) {
		return filepath.Join(def.root, filepath.FromSlash(r.Rel)), nil
	}
	return "", errors.New(i18n.T("未知的全局内容类别"))
}
