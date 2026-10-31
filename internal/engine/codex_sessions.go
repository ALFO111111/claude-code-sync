package engine

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// Codex 会话同步：$CODEX_HOME/sessions/YYYY/MM/DD/rollout-<时间>-<会话ID>.jsonl。
//
// 只同步这些 rollout 文件：Codex 恢复会话时从它重建上下文，SQLite（state_*.sqlite、thread_history_*.sqlite）
// 都是能自己重建的索引，拷过去反而会和本机的索引打架，所以一律不碰。
//
// 云端按设备分开放：tools/codex/sessions/<设备ID>/<相对路径>，每台设备只写自己那一份，
// 两台设备各自续写同一个会话时，不会出现"你盖我、我盖你"来回覆盖。拉别的设备的版本时：
//
//	本机没有          原样（翻译路径后）写一个新文件
//	本机是它的开头    另一台接着聊了：把多出来的行追加到末尾（只追加，绝不重写）
//	它是本机的开头    本机更新，什么都不做（本机那份会照常上传）
//	各写各的          对方那份另存成一个新会话（ID 由"原 ID + 对方设备"决定，以后对方再续写就追加到这份里）
//
// 追加之前先拿 Codex 自己的写锁（thread-writer-locks/<ID>.lock）：拿不到说明 Codex 正开着这个会话，本轮跳过。
// 从别的设备拉下来、本机没再改过的文件不会重新上传；在本机删掉（或被 Codex 归档挪走）的会话不再拉回来。

const codexSessionPrefix = "tools/codex/sessions/"

// errCodexBusy Codex 正在写这个会话，本轮不碰它。不算失败。
var errCodexBusy = i18n.Err("Codex 正在使用这个会话")

// errCodexUnbound 会话所在的项目本机还没对上，先留在云端，对上以后再拉。不算失败。
var errCodexUnbound = i18n.Err("会话所在的项目在本机还没有对上")

func (e *Engine) codexSessionsDir() string { return filepath.Join(e.cfg.CodexDir, "sessions") }

func (e *Engine) codexSessionsEnabled() bool {
	return e.cfg.ContentEnabledFor(config.ToolCodex, config.ContentSessions)
}

func (e *Engine) codexOwnKey(rel string) string {
	return codexSessionPrefix + e.cfg.DeviceID + "/" + rel
}

// codexRelOK 会话相对路径必须是 .../rollout-*.jsonl，且是安全的相对路径（不能写到 sessions 外面）。
func codexRelOK(rel string) bool {
	base := path.Base(rel)
	return strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, ".jsonl") &&
		!skipProfileRel(config.ToolCodex, config.ContentSessions, rel)
}

// scanCodexSessions 本机的会话文件：相对路径（/ 分隔）→ 文件信息。sessions 目录不存在不算错。
func (e *Engine) scanCodexSessions() (map[string]*localFile, error) {
	out := map[string]*localFile{}
	root := e.codexSessionsDir()
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	err := e.walkFn(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if !codexRelOK(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[rel] = &localFile{abs: p, size: info.Size(), mtime: info.ModTime().UnixNano()}
		return nil
	})
	return out, err
}

type codexMeta struct {
	ID  string
	Cwd string
}

// parseCodexMeta 第一行是 session_meta：{"type":"session_meta","payload":{"id":...,"cwd":...}}。
func parseCodexMeta(firstLine []byte) codexMeta {
	var v struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal(bytes.TrimSpace(firstLine), &v) != nil || v.Type != "session_meta" {
		return codexMeta{}
	}
	return codexMeta{ID: v.Payload.ID, Cwd: v.Payload.Cwd}
}

func readCodexMetaFile(p string) codexMeta {
	f, err := os.Open(p)
	if err != nil {
		return codexMeta{}
	}
	defer f.Close()
	line, _ := bufio.NewReaderSize(f, 64<<10).ReadBytes('\n')
	return parseCodexMeta(line)
}

func firstLine(data []byte) []byte {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return data[:i]
	}
	return data
}

// discoverCodex 给本机 Codex 会话所在的目录认项目身份（和 Claude 共用同一套项目绑定）：
// 这样设备信息里会带上这些项目，另一台设备才能把路径翻译成它那边的位置。
func (e *Engine) discoverCodex() {
	if !e.codexSessionsEnabled() {
		return
	}
	local, err := e.scanCodexSessions()
	if err != nil {
		return // 同步时还会再扫一遍，失败在那里报
	}
	seen := map[string]bool{}
	for _, lf := range local {
		cwd := readCodexMetaFile(lf.abs).Cwd
		k := strings.ToLower(fsutil.CleanPath(cwd))
		if cwd == "" || seen[k] || !filepath.IsAbs(cwd) {
			continue
		}
		seen[k] = true
		if e.bindingContaining(cwd) != nil {
			continue
		}
		if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
			continue // 目录已经不在了，不凭空建绑定
		}
		dir := e.localDirFor(cwd)
		pid, display := e.identify(cwd, dir)
		if e.cfg.Ignored(pid, dir) || e.st.Ignored[pid] || e.st.Bindings[pid] != nil {
			continue
		}
		if e.bind(pid, cwd, display, false) != nil {
			e.log.Printf("发现 Codex 项目 %s（%s）", cwd, pid)
		}
	}
}

// bindingContaining 本机绑定里包含这个路径的那个（最长的根优先）。
func (e *Engine) bindingContaining(p string) *state.Binding {
	var best *state.Binding
	for _, b := range e.st.Bindings {
		if _, ok := pathWithin(b.Root, p); ok && (best == nil || len(b.Root) > len(best.Root)) {
			best = b
		}
	}
	return best
}

// foreignWithin 按"不分大小写、\ 和 / 一样"判断其他设备上的路径 p 是否在 root 里（对方可能是另一种系统）。
func foreignWithin(root, p string) bool {
	norm := func(s string) string { return strings.TrimRight(strings.ToLower(strings.ReplaceAll(s, `\`, "/")), "/") }
	r, q := norm(root), norm(p)
	return r != "" && (q == r || strings.HasPrefix(q, r+"/"))
}

// codexTranslator 把 device 上的路径翻译成本机路径：会话所在项目的根 + 用户目录。
// 会话所在的项目本机还没对上时返回 errCodexUnbound（先不拉，免得写进去一堆本机不存在的路径）。
func (e *Engine) codexTranslator(device, cwd string) (*translator, error) {
	d, ok := e.devices[device]
	if !ok {
		return nil, errCodexUnbound // 设备信息还没传上来
	}
	var pairs [][2]string
	if cwd != "" {
		bestPID, bestRoot := "", ""
		for pid, p := range d.Projects {
			if foreignWithin(p.Root, cwd) && len(p.Root) > len(bestRoot) {
				bestPID, bestRoot = pid, p.Root
			}
		}
		b := e.st.Bindings[bestPID]
		if bestPID == "" || b == nil {
			return nil, errCodexUnbound
		}
		pairs = append(pairs, [2]string{bestRoot, b.Root})
	}
	if d.Home != "" {
		pairs = append(pairs, [2]string{d.Home, e.cfg.Home})
	}
	return newPairTranslator(pairs), nil
}

// codexForkID 分叉会话的新 ID：由"原 ID + 对方设备"决定（对方以后再续写，还对应同一份分叉）。
// 保留原 ID 前 48 位的时间部分并标成 v7，排序时还落在原会话附近。
func codexForkID(id, device string) string {
	sum := sha256.Sum256([]byte("github.com/ALFO111111/claude-code-sync/codex-fork\x00" + id + "\x00" + device))
	b := sum[:16]
	if orig, err := hex.DecodeString(strings.ReplaceAll(id, "-", "")); err == nil && len(orig) == 16 {
		copy(b[:6], orig[:6])
	}
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func isUUID(s string) bool {
	if len(s) != 36 || strings.Count(s, "-") != 4 {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return err == nil
}

// lockCodexThread 拿 Codex 的会话写锁（和 Codex 一样：先拿协调锁，再非阻塞地锁 <ID>.lock）。
// 会话正被 Codex 打开时返回 errCodexBusy。
func (e *Engine) lockCodexThread(id string) (func(), error) {
	dir := filepath.Join(e.cfg.CodexDir, "thread-writer-locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	coord, err := os.OpenFile(filepath.Join(dir, ".coordination.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	defer coord.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ok, err := fsutil.TryLockFile(coord)
		if err != nil {
			return nil, err
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			return nil, errCodexBusy
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer fsutil.UnlockFile(coord)
	f, err := os.OpenFile(filepath.Join(dir, id+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	ok, err := fsutil.TryLockFile(f)
	if err != nil || !ok {
		f.Close()
		if err == nil {
			err = errCodexBusy
		}
		return nil, err
	}
	return func() { fsutil.UnlockFile(f); f.Close() }, nil
}

// syncCodexSessions 一轮 Codex 会话同步：先传本机改过的，再拉别的设备的。
func (e *Engine) syncCodexSessions(ctx context.Context) error {
	if !e.codexSessionsEnabled() {
		return nil
	}
	objs, err := e.store.List(ctx, codexSessionPrefix)
	if err != nil {
		return fmt.Errorf(i18n.T("列出云端 Codex 会话失败: %w"), err)
	}
	local, err := e.scanCodexSessions()
	if err != nil {
		return fmt.Errorf(i18n.T("遍历 Codex 会话目录失败（本轮跳过 Codex 会话）: %w"), err)
	}
	type version struct {
		device string
		obj    *storage.Object
	}
	remote := map[string][]version{} // rel → 其他设备的版本
	for i := range objs {
		o := &objs[i]
		device, rel, ok := strings.Cut(strings.TrimPrefix(o.Key, codexSessionPrefix), "/")
		if !ok || device == e.cfg.DeviceID || device == "" || !codexRelOK(rel) {
			continue
		}
		remote[rel] = append(remote[rel], version{device, o})
	}
	forkRels := map[string]bool{}
	for _, r := range e.st.CodexForks {
		forkRels[r] = true
	}

	// 本机删掉的（或被 Codex 归档挪走的）：记下来，以后不再拉回
	ownPrefix := codexSessionPrefix + e.cfg.DeviceID + "/"
	if !e.dry {
		for k, s := range e.st.Files {
			if rel, ok := strings.CutPrefix(k, ownPrefix); ok && local[rel] == nil && !s.Deleted {
				e.mu.Lock()
				s.Deleted = true
				e.mu.Unlock()
			}
		}
	}

	rels := make([]string, 0, len(local))
	for rel := range local {
		if !forkRels[rel] {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)
	pulls := make([]string, 0, len(remote))
	for rel := range remote {
		pulls = append(pulls, rel)
	}
	sort.Strings(pulls)
	e.setTotal(len(rels) + len(pulls))

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, rel := range rels {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done(); e.step() }()
			if err := e.pushCodexSession(ctx, rel, local[rel]); err != nil {
				e.log.Printf("Codex 会话上传失败 %s: %v", rel, err)
				e.count(&e.stats.Errors)
			}
		}()
	}
	wg.Wait()

	// 拉取按文件顺序来：同一个会话的几个版本要依次比对（先比最长的，短的多半就是它的开头）
	for _, rel := range pulls {
		if ctx.Err() != nil {
			break
		}
		vs := remote[rel]
		sort.Slice(vs, func(i, j int) bool { return vs[i].obj.Size > vs[j].obj.Size })
		for _, v := range vs {
			switch err := e.pullCodexSession(ctx, rel, v.device, v.obj); {
			case errors.Is(err, errCodexBusy), errors.Is(err, errLocalChanged):
				e.log.Printf("Codex 会话 %s 正在使用，本轮先跳过", rel)
			case errors.Is(err, errCodexUnbound):
				// 项目在本机还没对上（待认领），对上后自动拉；不刷日志
			case err != nil:
				e.log.Printf("Codex 会话下载失败 %s: %v", rel, err)
				e.count(&e.stats.Errors)
			}
		}
		e.step()
	}
	return ctx.Err()
}

func (e *Engine) noteCodex(rel, kind string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := codexSessionPrefix + rel
	if old, ok := e.changes[key]; ok && changeRank[old.Kind] >= changeRank[kind] {
		return
	}
	e.changes[key] = state.Change{Kind: kind, Tool: config.ToolCodex, Dir: config.ContentSessions, Rel: rel}
}

// pushCodexSession 本机这份有变化就上传到自己名下（只传完整的行）。
func (e *Engine) pushCodexSession(ctx context.Context, rel string, lf *localFile) error {
	key := e.codexOwnKey(rel)
	e.mu.Lock()
	s := e.st.Files[key]
	e.mu.Unlock()
	if s != nil && !s.Deleted && s.Size == lf.size && s.Mtime == lf.mtime {
		return nil
	}
	data, err := os.ReadFile(lf.abs)
	if err != nil {
		return err
	}
	up := trimPartialLine(data)
	h := crypt.SHA256Hex(up)
	if s != nil && s.Hash == h {
		if !e.dry {
			e.setState(key, &state.FileState{ETag: s.ETag, Size: lf.size, Mtime: lf.mtime, Hash: h})
		}
		return nil
	}
	if len(up) == 0 {
		return nil
	}
	if e.dry {
		e.log.Printf("将上传 Codex 会话 %s", rel)
		e.count(&e.stats.Pushed)
		return nil
	}
	enc, err := crypt.Seal(e.key, key, up)
	if err != nil {
		return err
	}
	etag, err := e.store.Put(ctx, key, enc) // 只有本机写自己名下，不会被别人覆盖
	if err != nil {
		return err
	}
	size, mtime := lf.size, lf.mtime
	if len(up) != len(data) {
		mtime = 0 // 末尾半行没传：下轮一定再看一遍
	}
	e.setState(key, &state.FileState{ETag: etag, Size: size, Mtime: mtime, Hash: h})
	e.count(&e.stats.Pushed)
	e.noteCodex(rel, state.ChangePush)
	return nil
}

// recordCodexMirror 本机这份内容来自别的设备（或已经包含在别的设备的版本里）：记成"已同步"，不再上传。
func (e *Engine) recordCodexMirror(rel string, content []byte) error {
	fi, err := os.Stat(filepath.Join(e.codexSessionsDir(), filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	key := e.codexOwnKey(rel)
	e.mu.Lock()
	etag := ""
	if s := e.st.Files[key]; s != nil {
		etag = s.ETag
	}
	e.mu.Unlock()
	mtime := fi.ModTime().UnixNano()
	if fi.Size() != int64(len(content)) {
		mtime = 0 // 写完之后 Codex 又写了：下轮重新比对
	}
	e.setState(key, &state.FileState{ETag: etag, Size: fi.Size(), Mtime: mtime, Hash: crypt.SHA256Hex(trimPartialLine(content))})
	return nil
}

// pullCodexSession 把 device 上的这个会话版本合进本机（规则见文件开头）。
func (e *Engine) pullCodexSession(ctx context.Context, rel, device string, obj *storage.Object) error {
	unbound := codexUnboundMark + e.codexBindDigest(device)
	if s := e.st.Files[obj.Key]; s != nil && s.ETag == obj.ETag {
		if !strings.HasPrefix(s.Hash, codexUnboundMark) || s.Hash == unbound {
			return nil // 这个版本处理过了（或者还没对上、项目绑定也没变：不用每轮都重新下载一遍）
		}
	}
	err := e.pullCodexVersion(ctx, rel, device, obj)
	if errors.Is(err, errCodexUnbound) && !e.dry {
		e.setState(obj.Key, &state.FileState{ETag: obj.ETag, Size: obj.Size, Hash: unbound})
	}
	return err
}

// codexUnboundMark 状态里记"这个版本因为项目没对上先没拉"；后面跟着当时的绑定摘要，绑定一变就重试。
const codexUnboundMark = "unbound:"

// codexBindDigest 本机绑定了哪些项目 + 对方设备上报了哪些项目：决定一个会话能不能翻译路径。
func (e *Engine) codexBindDigest(device string) string {
	var ids []string
	for pid := range e.st.Bindings {
		ids = append(ids, "l:"+pid)
	}
	for pid, p := range e.devices[device].Projects {
		ids = append(ids, "r:"+pid+"="+p.Root)
	}
	sort.Strings(ids)
	return crypt.SHA256Hex([]byte(strings.Join(ids, "\n")))[:16]
}

func (e *Engine) pullCodexVersion(ctx context.Context, rel, device string, obj *storage.Object) error {
	forkKey := device + "/" + rel
	target := e.st.CodexForks[forkKey]
	if target == "" {
		target = rel
	}
	if s := e.st.Files[e.codexOwnKey(target)]; s != nil && s.Deleted {
		return nil // 本机删掉/归档了，不拉回
	}
	data, _, err := e.store.Get(ctx, obj.Key)
	if err != nil {
		return err
	}
	raw, err := e.open(obj.Key, data)
	if err != nil {
		return err
	}
	meta := parseCodexMeta(firstLine(raw))
	if !isUUID(meta.ID) || !strings.Contains(path.Base(rel), meta.ID) {
		return fmt.Errorf(i18n.T("不是有效的 Codex 会话文件: %s"), rel)
	}
	tr, err := e.codexTranslator(device, meta.Cwd)
	if err != nil {
		return err
	}
	content := tr.apply(trimPartialLine(raw))
	id := meta.ID
	if target != rel {
		id = codexForkID(meta.ID, device)
		content = bytes.ReplaceAll(content, []byte(meta.ID), []byte(id))
	}
	if e.dry {
		e.log.Printf("将比对 Codex 会话 %s（来自 %s）", rel, device)
		return nil
	}
	err = e.applyCodexContent(target, id, content)
	if errors.Is(err, errCodexDiverged) {
		if target != rel {
			// 分叉文件本身只会由对方单向追加；对不上说明被人改过，这份保持原样，新内容另行备份
			e.bk.saveGlobal(BackupRemote, config.ToolCodex, config.ContentSessions, target, content)
			e.log.Printf("Codex 会话分叉 %s 和 %s 上的版本对不上，对方的版本已放进备份", target, device)
			e.recordRemote(obj)
			return nil
		}
		forkID := codexForkID(meta.ID, device)
		forkRel := path.Join(path.Dir(rel), strings.Replace(path.Base(rel), meta.ID, forkID, 1))
		forked := bytes.ReplaceAll(content, []byte(meta.ID), []byte(forkID))
		if err := e.applyCodexContent(forkRel, forkID, forked); err != nil {
			return err
		}
		e.mu.Lock()
		e.st.CodexForks[forkKey] = forkRel
		e.mu.Unlock()
		e.log.Printf("Codex 会话 %s 在本机和 %s 上各自继续了，对方那份已另存为新会话 %s", meta.ID, device, forkID)
		e.count(&e.stats.Forked)
		e.noteCodex(forkRel, state.ChangeFork)
	} else if err != nil {
		return err
	}
	e.recordRemote(obj)
	return nil
}

// recordRemote 记下"这个远端版本已经合进来了"，下轮 etag 没变就不再下载。
func (e *Engine) recordRemote(obj *storage.Object) {
	e.setState(obj.Key, &state.FileState{ETag: obj.ETag, Size: obj.Size})
}

var errCodexDiverged = errors.New("codex session diverged")

// applyCodexContent 让本机 rel 这份文件"至少包含" content：没有就新建，是开头就追加，已经更长就不动；
// 两边各有对方没有的内容返回 errCodexDiverged。
func (e *Engine) applyCodexContent(rel, id string, content []byte) error {
	abs := filepath.Join(e.codexSessionsDir(), filepath.FromSlash(rel))
	cur, err := os.ReadFile(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := writeNewFile(abs, content); err != nil {
			return err
		}
		e.count(&e.stats.Pulled)
		e.noteCodex(rel, state.ChangePull)
		return e.recordCodexMirror(rel, content)
	case err != nil:
		return err
	}
	if !lastLineComplete(cur) {
		return errLocalChanged // Codex 正写到一半
	}
	switch {
	case bytes.Equal(cur, content):
		return e.recordCodexMirror(rel, cur)
	case bytes.HasPrefix(cur, content):
		return nil // 本机更新：照常上传本机这份
	case !bytes.HasPrefix(content, cur):
		return errCodexDiverged
	}
	unlock, err := e.lockCodexThread(id)
	if err != nil {
		return err
	}
	defer unlock()
	fi, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if fi.Size() != int64(len(cur)) {
		return errLocalChanged // 拿锁前刚被写过：下轮重新比对
	}
	merged, err := appendLines(abs, cur, SplitLines(content[len(cur):]))
	if err != nil {
		return err
	}
	e.count(&e.stats.Pulled)
	e.noteCodex(rel, state.ChangePull)
	return e.recordCodexMirror(rel, merged)
}

// writeNewFile 写一个原本不存在的文件；写的过程中被别人建出来了就放弃（不覆盖）。
func writeNewFile(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return errLocalChanged
	}
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(abs)
	}
	return werr
}
