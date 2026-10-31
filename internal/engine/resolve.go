package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ALFO111111/agent-sync/internal/claudeconfig"
	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/state"
)

const (
	ResolveLocal  = "local"
	ResolveRemote = "remote"
	ResolveMerged = "merged"
	ResolveBoth   = "both"
)

var conflictIDPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ResolveConflict 解决一个持久冲突。本机或云端在冲突产生后又有新版本时拒绝覆盖，保留冲突让用户重新比较。
func ResolveConflict(ctx context.Context, id, choice string, merged []byte) error {
	if !conflictIDPattern.MatchString(id) {
		return errors.New(i18n.T("非法冲突 ID"))
	}
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := LoadConflict(id)
	if err != nil {
		return err
	}
	blobs, err := ReadConflictBlobs(id)
	if err != nil {
		return err
	}
	e, err := New(true)
	if err != nil {
		return err
	}
	if e.st, err = state.Load(); err != nil {
		return err
	}
	e.changes = map[string]state.Change{}
	it := &profileItem{key: c.Key, tool: c.Tool, category: c.Category, rel: c.Rel, kind: c.Kind, st: e.st.Files[c.Key]}
	abs := e.profileLocalPath(it)
	fi, statErr := os.Stat(abs)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	it.local = &localFile{abs: abs, size: -1}
	if statErr == nil {
		it.local.size, it.local.mtime = fi.Size(), fi.ModTime().UnixNano()
	}
	current, err := e.readProfileLocal(it)
	if err != nil {
		return err
	}
	if crypt.SHA256Hex(current) != c.LocalHash {
		return errors.New(i18n.T("本机文件在冲突产生后又改过，请先重新同步再处理"))
	}
	remote, etag, err := e.fetchProfileRemote(ctx, it)
	if err != nil {
		return err
	}
	if c.RemoteETag != "" && etag != c.RemoteETag || !bytes.Equal(remote, blobs.Remote) {
		return errors.New(i18n.T("云端文件在冲突产生后又改过，请先重新同步再处理"))
	}
	var data []byte
	switch choice {
	case ResolveLocal:
		data = blobs.Local
	case ResolveRemote:
		data = blobs.Remote
	case ResolveMerged:
		data = merged
		if it.kind == ConflictSettings {
			if err := claudeconfig.ValidateSettingsProjection(data); err != nil {
				return fmt.Errorf(i18n.T("手工合并的 settings 不符合格式: %w"), err)
			}
		}
		if it.kind == ConflictMCP {
			if err := claudeconfig.ValidateMCPProjection(data); err != nil {
				return fmt.Errorf(i18n.T("手工合并的 MCP 不符合格式: %w"), err)
			}
		}
	case ResolveBoth:
		if it.category == config.ContentRules || it.kind != ConflictFile && it.kind != ConflictTasks {
			return errors.New(i18n.T("这个文件不能保留两份，请手工合并"))
		}
		return e.resolveBoth(ctx, it, c, blobs)
	default:
		return errors.New(i18n.T("未知的冲突解决方式"))
	}
	if choice != ResolveLocal {
		if err := e.applyProfileLocal(it, data); err != nil {
			return err
		}
	}
	if err := e.pushProfile(ctx, it, data); err != nil {
		return err
	}
	// putVerified 发现并发覆盖时不会更新 state，此时冲突也必须保留。
	if st := e.st.Files[c.Key]; st == nil || st.Hash != crypt.SHA256Hex(data) {
		return errors.New(i18n.T("云端刚被其他设备覆盖，冲突尚未解决，请重试"))
	}
	if err := state.Save(e.st); err != nil {
		return err
	}
	return deleteConflict(id)
}

func (e *Engine) resolveBoth(ctx context.Context, it *profileItem, c *Conflict, blobs *ConflictBlobs) error {
	if err := e.pushProfile(ctx, it, blobs.Local); err != nil {
		return err
	}
	if st := e.st.Files[c.Key]; st == nil || st.Hash != crypt.SHA256Hex(blobs.Local) {
		return errors.New(i18n.T("云端刚被其他设备覆盖，冲突尚未解决，请重试"))
	}
	copyRel := conflictCopyRel(it.rel, e.cfg.DeviceID, blobs.Remote)
	copyItem := &profileItem{key: profileFileKey(it.tool, it.category, copyRel),
		tool: it.tool, category: it.category, rel: copyRel, kind: it.kind,
		local: &localFile{abs: filepath.Join(filepath.Dir(it.local.abs), filepath.Base(filepath.FromSlash(copyRel))), size: -1}}
	if _, err := os.Stat(copyItem.local.abs); !errors.Is(err, fs.ErrNotExist) {
		return errors.New(i18n.T("保留两份用的副本文件已经存在，请先重命名"))
	}
	if err := fsutil.WriteFileAtomic(copyItem.local.abs, blobs.Remote); err != nil {
		return err
	}
	fi, err := os.Stat(copyItem.local.abs)
	if err != nil {
		return err
	}
	copyItem.local.size, copyItem.local.mtime = fi.Size(), fi.ModTime().UnixNano()
	if err := e.pushProfile(ctx, copyItem, blobs.Remote); err != nil {
		return err
	}
	if err := state.Save(e.st); err != nil {
		return err
	}
	return deleteConflict(c.ID)
}

func conflictCopyRel(rel, deviceID string, remote []byte) string {
	ext := filepath.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	device := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, deviceID)
	if device == "" {
		device = "device"
	}
	return fmt.Sprintf("%s.%s.%s%s", base, device, crypt.SHA256Hex(remote)[:8], ext)
}

// RestoreContentItem 手工恢复一个已删除/忽略的 Claude 全局内容项。
func RestoreContentItem(ctx context.Context, category, rel string) error {
	return RestoreToolContentItem(ctx, config.ToolClaude, category, rel)
}

// RestoreToolContentItem 从云端拉回并清除 tombstone。
func RestoreToolContentItem(ctx context.Context, tool, category, rel string) error {
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	e, err := New(true)
	if err != nil {
		return err
	}
	if e.st, err = state.Load(); err != nil {
		return err
	}
	e.changes = map[string]state.Change{}
	def, known := findProfileDef(e.cfg, tool, category)
	if !known || !def.allows(rel) || skipProfileRel(tool, category, rel) || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return errors.New(i18n.T("非法全局内容路径"))
	}
	key := profileFileKey(tool, category, rel)
	it := &profileItem{key: key, tool: tool, category: category, rel: rel, kind: def.kind, st: e.st.Files[key]}
	remote, etag, err := e.fetchProfileRemote(ctx, it)
	if err != nil {
		return err
	}
	abs := e.profileLocalPath(it)
	if _, err := os.Stat(abs); !errors.Is(err, fs.ErrNotExist) {
		return errors.New(i18n.T("本机文件已经存在，请先同步或处理冲突"))
	}
	if err := e.pullProfile(ctx, it, remote, etag); err != nil {
		return err
	}
	return state.Save(e.st)
}
