package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ALFO111111/claude-code-sync/internal/ccswitch"
	"github.com/ALFO111111/claude-code-sync/internal/claudeconfig"
	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

const profileFilePrefix = "tools/claude/files/"
const profileConfigPrefix = "tools/claude/config/"
const profileSettingsKey = profileConfigPrefix + "settings.json"
const profileMCPKey = profileConfigPrefix + "mcp.json"

func profileFilePrefixFor(tool string) string { return "tools/" + tool + "/files/" }

func profileFileKey(tool, category, rel string) string {
	return profileFilePrefixFor(tool) + category + "/" + rel
}

type profileDef struct {
	tool     string
	category string
	root     string
	kind     string
	files    []string // 非空时只同步这些单文件；空表示递归同步整个 root
}

func (d profileDef) allows(rel string) bool {
	if len(d.files) == 0 {
		return true
	}
	for _, name := range d.files {
		if rel == name {
			return true
		}
	}
	return false
}

type profileItem struct {
	key      string
	tool     string
	category string
	rel      string
	kind     string
	local    *localFile
	remote   *storage.Object
	st       *state.FileState
}

func profileDefsFor(cfg *config.Config) []profileDef {
	return []profileDef{
		{tool: config.ToolClaude, category: config.ContentRules, root: cfg.ClaudeDir, kind: ConflictFile, files: []string{"CLAUDE.md"}},
		{tool: config.ToolClaude, category: config.ContentPlans, root: filepath.Join(cfg.ClaudeDir, "plans"), kind: ConflictFile},
		{tool: config.ToolClaude, category: config.ContentSkills, root: filepath.Join(cfg.ClaudeDir, "skills"), kind: ConflictFile},
		{tool: config.ToolClaude, category: config.ContentAgents, root: filepath.Join(cfg.ClaudeDir, "agents"), kind: ConflictFile},
		{tool: config.ToolClaude, category: config.ContentCommands, root: filepath.Join(cfg.ClaudeDir, "commands"), kind: ConflictFile},
		{tool: config.ToolClaude, category: config.ContentTasks, root: filepath.Join(cfg.ClaudeDir, "tasks"), kind: ConflictTasks},
		{tool: config.ToolCodex, category: config.ContentRules, root: cfg.CodexDir, kind: ConflictFile, files: []string{"AGENTS.md", "AGENTS.override.md"}},
		{tool: config.ToolCodex, category: config.ContentSkills, root: filepath.Join(cfg.Home, ".agents", "skills"), kind: ConflictFile},
		{tool: config.ToolCodex, category: config.ContentLegacySkills, root: filepath.Join(cfg.CodexDir, "skills"), kind: ConflictFile},
	}
}

func (e *Engine) profileDefs() []profileDef { return profileDefsFor(e.cfg) }

func findProfileDef(cfg *config.Config, tool, category string) (profileDef, bool) {
	if tool == "" {
		tool = config.ToolClaude
	}
	for _, def := range profileDefsFor(cfg) {
		if def.tool == tool && def.category == category {
			return def, true
		}
	}
	return profileDef{}, false
}

// syncProfile 同步 Claude 的全局个人内容。会话 projects 仍走原有 syncFiles，互不混用合并规则。
func (e *Engine) syncProfile(ctx context.Context) error {
	ccEnabled := false
	switch ccswitch.DetectAt(e.cfg.Home) {
	case ccswitch.Enabled:
		ccEnabled = true
		if e.cfg.UpdateCCSwitch(true) && !e.dry {
			if err := config.Save(e.cfg); err != nil {
				e.log.Printf("保存 CC Switch 检测状态失败: %v", err)
			}
		}
	case ccswitch.Disabled, ccswitch.NotInstalled:
		if e.cfg.UpdateCCSwitch(false) && !e.dry {
			if err := config.Save(e.cfg); err != nil {
				e.log.Printf("保存 CC Switch 检测状态失败: %v", err)
			}
		}
	}
	active := map[string]bool{}
	for _, def := range e.profileDefs() {
		if e.cfg.ContentEnabledFor(def.tool, def.category) &&
			(def.tool != config.ToolClaude || !e.cfg.ContentPending(def.category, ccEnabled)) {
			active[config.ContentKey(def.tool, def.category)] = true
		}
	}
	for _, category := range []string{config.ContentSettings, config.ContentMCP} {
		if e.cfg.ContentEnabled(category) && !e.cfg.ContentPending(category, ccEnabled) {
			active[category] = true
		}
	}
	if len(active) == 0 {
		return nil
	}
	items, err := e.collectProfileItems(ctx, active)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(items))
	for key, it := range items {
		it.st = e.st.Files[key]
		keys = append(keys, key)
	}
	sort.Strings(keys)
	e.setTotal(len(keys))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, key := range keys {
		it := items[key]
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done(); e.step() }()
			switch err := e.syncProfileItem(ctx, it); {
			case errors.Is(err, errLocalChanged):
				e.log.Printf("全局文件 %s 正在被修改，本轮先跳过", it.localPath())
			case err != nil:
				e.log.Printf("全局文件同步失败 %s/%s: %v", it.category, it.rel, err)
				e.count(&e.stats.Errors)
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}

func (e *Engine) collectProfileItems(ctx context.Context, active map[string]bool) (map[string]*profileItem, error) {
	items := map[string]*profileItem{}
	defs := e.profileDefs()
	for _, tool := range []string{config.ToolClaude, config.ToolCodex} {
		needsList := false
		for _, def := range defs {
			if def.tool == tool && active[config.ContentKey(tool, def.category)] {
				needsList = true
				break
			}
		}
		if !needsList {
			continue
		}
		prefix := profileFilePrefixFor(tool)
		objs, err := e.store.List(ctx, prefix)
		if err != nil {
			return nil, fmt.Errorf(i18n.T("列出云端全局文件失败: %w"), err)
		}
		for i := range objs {
			o := &objs[i]
			rest := strings.TrimPrefix(o.Key, prefix)
			category, rel, ok := strings.Cut(rest, "/")
			def, known := findProfileDef(e.cfg, tool, category)
			if !ok || !known || !active[config.ContentKey(tool, category)] || !def.allows(rel) || skipProfileRel(tool, category, rel) {
				continue
			}
			items[o.Key] = &profileItem{key: o.Key, tool: tool, category: category, rel: rel, kind: def.kind, remote: o}
		}
	}
	for _, def := range defs {
		if !active[config.ContentKey(def.tool, def.category)] {
			continue
		}
		if err := e.collectProfileLocal(items, def); err != nil {
			e.log.Printf("遍历全局内容 %s 失败（本类别本轮跳过）: %v", def.root, err)
			e.count(&e.stats.Errors)
			for key, it := range items {
				if it.tool == def.tool && it.category == def.category {
					delete(items, key)
				}
			}
		}
	}
	if active[config.ContentSettings] || active[config.ContentMCP] {
		if err := e.collectProfileConfig(ctx, items, active); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (e *Engine) collectProfileConfig(ctx context.Context, items map[string]*profileItem, active map[string]bool) error {
	objs, err := e.store.List(ctx, profileConfigPrefix)
	if err != nil {
		return fmt.Errorf(i18n.T("列出云端结构化配置失败: %w"), err)
	}
	for i := range objs {
		o := &objs[i]
		var category, kind, rel string
		switch o.Key {
		case profileSettingsKey:
			category, kind, rel = config.ContentSettings, ConflictSettings, "settings.json"
		case profileMCPKey:
			category, kind, rel = config.ContentMCP, ConflictMCP, "mcp.json"
		default:
			continue
		}
		if active[category] {
			items[o.Key] = &profileItem{key: o.Key, tool: config.ToolClaude, category: category, rel: rel, kind: kind, remote: o}
		}
	}
	for _, def := range []struct {
		active   bool
		key      string
		category string
		kind     string
		rel      string
		abs      string
	}{
		{active[config.ContentSettings], profileSettingsKey, config.ContentSettings, ConflictSettings, "settings.json", filepath.Join(e.cfg.ClaudeDir, "settings.json")},
		{active[config.ContentMCP], profileMCPKey, config.ContentMCP, ConflictMCP, "mcp.json", e.cfg.ClaudeStatePath()},
	} {
		if !def.active {
			continue
		}
		it := items[def.key]
		if it == nil {
			it = &profileItem{key: def.key, tool: config.ToolClaude, category: def.category, rel: def.rel, kind: def.kind}
			items[def.key] = it
		}
		fi, err := os.Stat(def.abs)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		lf := &localFile{abs: def.abs, size: -1}
		if err == nil {
			lf.size, lf.mtime = fi.Size(), fi.ModTime().UnixNano()
		}
		it.local = lf
	}
	return nil
}

func (e *Engine) collectProfileLocal(items map[string]*profileItem, def profileDef) error {
	add := func(path, rel string, info fs.FileInfo) {
		key := profileFileKey(def.tool, def.category, rel)
		it := items[key]
		if it == nil {
			it = &profileItem{key: key, tool: def.tool, category: def.category, rel: rel, kind: def.kind}
			items[key] = it
		}
		it.local = &localFile{abs: path, size: info.Size(), mtime: info.ModTime().UnixNano()}
	}
	if len(def.files) > 0 {
		for _, rel := range def.files {
			abs := filepath.Join(def.root, filepath.FromSlash(rel))
			fi, err := os.Lstat(abs)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if !fi.IsDir() && !skipFile(fi.Name()) {
				add(abs, rel, fi)
			}
		}
		return nil
	}
	fi, err := os.Stat(def.root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		if skipFile(fi.Name()) {
			return nil
		}
		add(def.root, filepath.Base(def.root), fi)
		return nil
	}
	return e.walkFn(def.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(def.root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if path != def.root && skipProfileRel(def.tool, def.category, rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Type()&os.ModeSymlink != 0 {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		add(path, rel, info)
		return nil
	})
}

func skipProfileRel(tool, category, rel string) bool {
	// 云端相对路径一律用 / 分隔：出现 \ 或 : 就是被构造的（Windows 盘符、UNC、..\ 逃逸），
	// 不能只靠 filepath.IsLocal——在 macOS 上它把 C:/x、..\x 当成普通文件名放行。
	if strings.ContainsAny(rel, `\:`) || strings.HasPrefix(rel, "/") || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return true
	}
	parts := strings.Split(strings.ToLower(rel), "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return true
		}
		if tool == config.ToolClaude && category == config.ContentSkills && (part == "synced" || part == ".trash") {
			return true
		}
		// $CODEX_HOME/skills/.system 是 Codex 自带的系统技能，每次启动按指纹重装，不同步
		if tool == config.ToolCodex && category == config.ContentLegacySkills && i == 0 && part == ".system" {
			return true
		}
	}
	return skipFile(parts[len(parts)-1])
}

func (it *profileItem) localPath() string {
	if it.local != nil {
		return it.local.abs
	}
	return ""
}

func (e *Engine) profileLocalPath(it *profileItem) string {
	if it.local != nil {
		return it.local.abs
	}
	if it.kind == ConflictSettings {
		return filepath.Join(e.cfg.ClaudeDir, "settings.json")
	}
	if it.kind == ConflictMCP {
		return e.cfg.ClaudeStatePath()
	}
	for _, def := range e.profileDefs() {
		if def.tool == it.tool && def.category == it.category {
			return filepath.Join(def.root, filepath.FromSlash(it.rel))
		}
	}
	return ""
}

func (e *Engine) readProfileLocal(it *profileItem) ([]byte, error) {
	if it.local == nil {
		return nil, nil
	}
	data, err := os.ReadFile(it.local.abs)
	if errors.Is(err, fs.ErrNotExist) && (it.kind == ConflictSettings || it.kind == ConflictMCP) {
		data = []byte("{}")
	} else if err != nil {
		return nil, err
	}
	switch it.kind {
	case ConflictSettings:
		return claudeconfig.SettingsProjection(data)
	case ConflictMCP:
		projection, skipped, err := claudeconfig.MCPProjection(data, e.mcpToCloud)
		if err == nil && len(skipped) > 0 {
			e.log.Printf("%s", claudeconfig.DescribeSkipped("MCP", skipped))
		}
		return projection, err
	default:
		return data, nil
	}
}

func (e *Engine) fetchProfileRemote(ctx context.Context, it *profileItem) ([]byte, string, error) {
	data, etag, err := e.store.Get(ctx, it.key)
	if err != nil {
		return nil, "", err
	}
	plain, err := e.open(it.key, data)
	return plain, etag, err
}

func (e *Engine) syncProfileItem(ctx context.Context, it *profileItem) error {
	s := it.st
	if it.local == nil {
		if it.remote == nil {
			return nil
		}
		if s != nil {
			if !s.Deleted && !e.dry {
				e.mu.Lock()
				s.Deleted = true
				e.mu.Unlock()
			}
			return nil
		}
		remote, etag, err := e.fetchProfileRemote(ctx, it)
		if err != nil {
			return err
		}
		return e.pullProfile(ctx, it, remote, etag)
	}
	local, err := e.readProfileLocal(it)
	if err != nil {
		return err
	}
	if it.remote == nil {
		if (it.kind == ConflictSettings || it.kind == ConflictMCP) && emptyProfileJSON(local) {
			return nil
		}
		return e.pushProfile(ctx, it, local)
	}
	localHash := crypt.SHA256Hex(local)
	if s != nil && !s.Deleted && s.Hash == localHash && s.ETag == it.remote.ETag {
		return nil
	}
	if s != nil && s.ETag == it.remote.ETag && (s.Hash != localHash || s.Deleted) {
		return e.pushProfile(ctx, it, local)
	}
	remote, etag, err := e.fetchProfileRemote(ctx, it)
	if err != nil {
		return err
	}
	if bytes.Equal(local, remote) {
		return e.recordProfile(it, local, etag)
	}
	base, hasBase := loadBase(it.key, s)
	if hasBase {
		switch {
		case bytes.Equal(local, base):
			return e.pullProfile(ctx, it, remote, etag)
		case bytes.Equal(remote, base):
			return e.pushProfile(ctx, it, local)
		}
	} else if s != nil && s.Hash == localHash && !s.Deleted {
		return e.pullProfile(ctx, it, remote, etag)
	} else if s == nil && emptyProfileJSON(local) && !emptyProfileJSON(remote) {
		return e.pullProfile(ctx, it, remote, etag)
	} else if s == nil && emptyProfileJSON(remote) && !emptyProfileJSON(local) {
		return e.pushProfile(ctx, it, local)
	}
	return e.mergeProfile(ctx, it, base, hasBase, local, remote, etag)
}

func emptyProfileJSON(data []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, key := range []string{"settings", "mcpServers"} {
			if nested, ok := x[key].(map[string]any); ok {
				return len(nested) == 0
			}
		}
		return len(x) == 0
	case []any:
		return len(x) == 0
	}
	return false
}

func (e *Engine) mergeProfile(ctx context.Context, it *profileItem, base []byte, hasBase bool, local, remote []byte, etag string) error {
	var merged []byte
	var conflict bool
	var err error
	if hasBase {
		switch {
		case it.kind == ConflictSettings || it.kind == ConflictMCP:
			merged, conflict, err = mergeJSON3(base, local, remote, false)
		case it.kind == ConflictTasks && strings.HasSuffix(strings.ToLower(it.rel), ".json"):
			merged, conflict, err = mergeJSON3(base, local, remote, true)
		case strings.HasSuffix(strings.ToLower(it.rel), ".json"):
			merged, conflict, err = mergeJSON3(base, local, remote, false)
		default:
			merged, conflict = mergeText3(base, local, remote)
		}
	} else {
		conflict = true
	}
	if err != nil {
		conflict = true
	}
	if conflict {
		return e.conflictProfile(it, base, hasBase, local, remote, etag)
	}
	if e.dry {
		e.log.Printf("将合并全局文件 %s/%s", it.category, it.rel)
		e.count(&e.stats.Merged)
		return nil
	}
	if err := e.applyProfileLocal(it, merged); err != nil {
		return err
	}
	e.count(&e.stats.Merged)
	e.noteProfile(it, state.ChangeMerge)
	return e.pushProfile(ctx, it, merged)
}

func (e *Engine) conflictProfile(it *profileItem, base []byte, hasBase bool, local, remote []byte, etag string) error {
	if e.dry {
		e.log.Printf("全局文件 %s/%s 需要人工处理冲突", it.category, it.rel)
		return nil
	}
	baseHash := ""
	if hasBase {
		baseHash = crypt.SHA256Hex(base)
	}
	c := &Conflict{Key: it.key, Tool: it.tool, Kind: it.kind, Category: it.category, Rel: it.rel,
		LocalPath: e.profileLocalPath(it), LocalHash: crypt.SHA256Hex(local), RemoteETag: etag, BaseHash: baseHash}
	id := conflictID(it.key)
	if _, err := LoadConflict(id); err == nil {
		if oldBlobs, err := ReadConflictBlobs(id); err == nil && bytes.Equal(oldBlobs.Base, base) && bytes.Equal(oldBlobs.Local, local) && bytes.Equal(oldBlobs.Remote, remote) {
			e.noteProfile(it, state.ChangeConflict)
			return nil
		}
	}
	if err := saveConflict(c, ConflictBlobs{Base: base, Local: local, Remote: remote}); err != nil {
		return err
	}
	e.log.Printf("全局文件 %s/%s 两边都有不同修改，已放入冲突中心", it.category, it.rel)
	e.noteProfile(it, state.ChangeConflict)
	return nil
}

func (e *Engine) applyProfileLocal(it *profileItem, data []byte) error {
	abs := e.profileLocalPath(it)
	fi, err := os.Stat(abs)
	localAbsent := it.local != nil && it.local.size < 0 && (it.kind == ConflictSettings || it.kind == ConflictMCP)
	switch {
	case it.local == nil && err == nil:
		return errLocalChanged
	case it.local == nil && !errors.Is(err, fs.ErrNotExist):
		return err
	case localAbsent && err == nil:
		return errLocalChanged
	case localAbsent && err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	case it.local != nil && errors.Is(err, fs.ErrNotExist) && !localAbsent:
		return errLocalChanged
	case it.local != nil && err != nil && !localAbsent:
		return err
	case it.local != nil && !localAbsent && (fi.Size() != it.local.size || fi.ModTime().UnixNano() != it.local.mtime):
		return errLocalChanged
	}
	var next []byte
	if it.kind == ConflictSettings || it.kind == ConflictMCP {
		cur := []byte("{}")
		if it.local.size >= 0 {
			cur, err = os.ReadFile(abs)
			if err != nil {
				return err
			}
		}
		oldProjection, err := e.readProfileLocal(it)
		if err != nil {
			return err
		}
		e.bk.saveGlobal(BackupLocal, it.tool, it.category, it.rel, oldProjection)
		switch it.kind {
		case ConflictSettings:
			next, err = claudeconfig.ApplySettings(cur, data)
		case ConflictMCP:
			var skipped []string
			next, skipped, err = claudeconfig.ApplyMCP(cur, data, e.mcpToLocal)
			if len(skipped) > 0 {
				e.log.Printf("%s", claudeconfig.DescribeSkipped("MCP", skipped))
			}
		}
		if err != nil {
			return err
		}
	} else {
		if it.local != nil {
			cur, err := os.ReadFile(abs)
			if err != nil {
				return err
			}
			e.bk.saveGlobal(BackupLocal, it.tool, it.category, it.rel, cur)
		}
		next = data
	}
	if err := fsutil.WriteFileAtomic(abs, next); err != nil {
		return err
	}
	fi, err = os.Stat(abs)
	if err != nil {
		return err
	}
	it.local = &localFile{abs: abs, size: fi.Size(), mtime: fi.ModTime().UnixNano()}
	return nil
}

func (e *Engine) pullProfile(ctx context.Context, it *profileItem, remote []byte, etag string) error {
	if e.dry {
		e.log.Printf("将下载全局文件 %s/%s", it.category, it.rel)
		e.count(&e.stats.Pulled)
		return nil
	}
	if it.local != nil {
		local, err := e.readProfileLocal(it)
		if err != nil {
			return err
		}
		if bytes.Equal(local, remote) {
			return e.recordProfile(it, local, etag)
		}
	}
	if err := e.applyProfileLocal(it, remote); err != nil {
		return err
	}
	if err := e.recordProfile(it, remote, etag); err != nil {
		return err
	}
	e.count(&e.stats.Pulled)
	e.noteProfile(it, state.ChangePull)
	return nil
}

func (e *Engine) pushProfile(ctx context.Context, it *profileItem, data []byte) error {
	if e.dry {
		e.log.Printf("将上传全局文件 %s/%s", it.category, it.rel)
		e.count(&e.stats.Pushed)
		return nil
	}
	enc, err := crypt.Seal(e.key, it.key, data)
	if err != nil {
		return err
	}
	etag, ok, err := e.putVerified(ctx, it.key, enc)
	if err != nil {
		return err
	}
	if !ok {
		e.log.Printf("云端全局文件 %s/%s 被其他设备并发覆盖，下轮重新合并", it.category, it.rel)
		return nil
	}
	if err := e.recordProfile(it, data, etag); err != nil {
		return err
	}
	e.count(&e.stats.Pushed)
	e.noteProfile(it, state.ChangePush)
	return nil
}

func (e *Engine) recordProfile(it *profileItem, data []byte, etag string) error {
	if e.dry {
		return nil
	}
	baseHash, err := saveBase(it.key, data)
	if err != nil {
		return err
	}
	if etag == "" && it.remote != nil {
		etag = it.remote.ETag
	}
	var size, mtime int64
	if it.local != nil {
		size, mtime = it.local.size, it.local.mtime
	}
	e.setState(it.key, &state.FileState{ETag: etag, Size: size, Mtime: mtime, Hash: crypt.SHA256Hex(data), BaseHash: baseHash})
	if err := deleteConflict(conflictID(it.key)); err != nil {
		e.log.Printf("清理已解决冲突记录失败 %s/%s: %v", it.category, it.rel, err)
	}
	return nil
}

func (e *Engine) noteProfile(it *profileItem, kind string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if old, ok := e.changes[it.key]; ok && changeRank[old.Kind] >= changeRank[kind] {
		return
	}
	e.changes[it.key] = state.Change{Kind: kind, Tool: it.tool, Dir: it.category, Rel: it.rel}
}
