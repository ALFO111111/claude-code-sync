package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/fsutil"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/project"
	"github.com/ALFO111111/agent-sync/internal/state"
	"github.com/ALFO111111/agent-sync/internal/storage"
)

// 文件同步：逐个比对本地和云端文件，决定上传、下载、合并还是分叉。
//
// 写本机文件的两条铁律（Claude Code 随时可能在写，不能吞掉它刚写的内容）：
//   - 会话（.jsonl）只往末尾追加缺的记录，绝不整份重写（和输入历史一样，见 history.go）
//   - 其他文件要整份替换时，替换前再看一眼：和比对时不一样了（刚被改过）就放弃这一轮，下轮重新比对；
//     替换掉的本机旧版本先进备份（见 backup.go）

// errLocalChanged 要写的本机文件在同步过程中又被改了：本轮跳过它，下轮重新比对。不算失败。
var errLocalChanged = i18n.Err("本机文件刚被修改")

func (e *Engine) translatorFor(b *state.Binding) *translator {
	e.mu.Lock()
	defer e.mu.Unlock()
	t, ok := e.translators[b.PID]
	if !ok {
		t = newTranslator(e.ForeignRoots(b.PID), b.Root)
		e.translators[b.PID] = t
	}
	return t
}

type localFile struct {
	abs   string
	size  int64
	mtime int64
}

type item struct {
	key    string
	rel    string
	b      *state.Binding
	local  *localFile
	remote *storage.Object
	st     *state.FileState
}

func skipFile(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".lock") || strings.HasSuffix(l, ".tmp") ||
		strings.HasPrefix(l, fsutil.TempPrefix) || strings.Contains(l, ".conflict")
}

func (e *Engine) syncFiles(ctx context.Context, only *state.Binding) error {
	byHash := map[string]*state.Binding{}
	for _, b := range e.st.Bindings {
		if only == nil || b == only {
			byHash[project.Hash(b.PID)] = b
		}
	}
	prefix := "files/"
	if only != nil {
		prefix += project.Hash(only.PID) + "/"
	}
	objs, err := e.store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf(i18n.T("列出云端文件失败: %w"), err)
	}

	items := map[string]*item{}
	for i := range objs {
		o := &objs[i]
		rest := strings.TrimPrefix(o.Key, "files/")
		h, rel, ok := strings.Cut(rest, "/")
		if b := byHash[h]; ok && b != nil {
			items[o.Key] = &item{key: o.Key, rel: rel, b: b, remote: o}
		}
	}
	// 遍历失败的绑定：本地文件清单不可信，整个项目本轮跳过。
	// 否则磁盘上还在的文件会被当成"本地没有"，导致误标 Deleted 或直接覆盖拉取。
	walkFailed := map[*state.Binding]bool{}
	for h, b := range byHash {
		dir := filepath.Join(e.cfg.ProjectsDir(), b.Dir)
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			// 目录不存在不算错误：本机从没在这个项目里聊过（典型：
			// 新机器刚自动绑定了别的设备的项目），当作空目录，云端文件正常下载。
			continue
		}
		walkErr := e.walkFn(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || skipFile(d.Name()) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			rel = filepath.ToSlash(rel)
			key := "files/" + h + "/" + rel
			it := items[key]
			if it == nil {
				it = &item{key: key, rel: rel, b: b}
				items[key] = it
			}
			it.local = &localFile{abs: p, size: info.Size(), mtime: info.ModTime().UnixNano()}
			return nil
		})
		if walkErr != nil {
			walkFailed[b] = true
			e.log.Printf("遍历 %s 失败（本次跳过该项目）: %v", dir, walkErr)
			e.count(&e.stats.Errors)
			for k, it := range items {
				if it.b == b {
					delete(items, k)
				}
			}
		}
	}

	// 两边都没有的文件，清掉状态记录
	if !e.dry {
		for k := range e.st.Files {
			h, _, _ := strings.Cut(strings.TrimPrefix(k, "files/"), "/")
			if b := byHash[h]; b != nil && !walkFailed[b] && items[k] == nil {
				delete(e.st.Files, k)
			}
		}
	}

	keys := make([]string, 0, len(items))
	for k, it := range items {
		it.st = e.st.Files[k]
		keys = append(keys, k)
	}
	sort.Strings(keys)
	e.setTotal(len(keys))

	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, k := range keys {
		it := items[k]
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done(); e.step() }()
			switch err := e.syncItem(ctx, it); {
			case errors.Is(err, errLocalChanged):
				e.log.Printf("%s/%s 正在被修改，本轮先跳过，下次同步再处理", it.b.Dir, it.rel)
			case err != nil:
				e.log.Printf("失败 %s/%s: %v", it.b.Dir, it.rel, err)
				e.count(&e.stats.Errors)
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}

func (e *Engine) syncItem(ctx context.Context, it *item) error {
	s := it.st
	switch {
	case it.local == nil && it.remote != nil:
		// 本地删了（比如 Claude Code 自动清理）并标记过的，不管远端怎么变都不拉回
		if s != nil && s.Deleted {
			return nil
		}
		if s != nil && s.ETag == it.remote.ETag {
			// 云端没变、本地被删 → 记住，不拉回来
			if !e.dry {
				e.mu.Lock()
				s.Deleted = true
				e.mu.Unlock()
			}
			return nil
		}
		return e.pull(ctx, it, nil, "")

	case it.local != nil && it.remote == nil:
		return e.push(ctx, it, nil)

	case it.local != nil && it.remote != nil:
		localChanged, data, err := e.localChanged(it)
		if err != nil {
			return err
		}
		if s == nil {
			return e.merge(ctx, it, data)
		}
		remoteChanged := s.ETag != it.remote.ETag
		switch {
		case !localChanged && !remoteChanged:
			return nil
		case localChanged && !remoteChanged:
			return e.push(ctx, it, data)
		case !localChanged && remoteChanged:
			if isSession(it.rel) {
				// 会话不整份覆盖：按记录比对，只把缺的追加到末尾（merge 里"本机是子集"那一支）
				return e.merge(ctx, it, data)
			}
			return e.pull(ctx, it, nil, "")
		default:
			return e.merge(ctx, it, data)
		}
	}
	return nil
}

func isSession(rel string) bool { return strings.HasSuffix(rel, ".jsonl") }

func (e *Engine) localChanged(it *item) (bool, []byte, error) {
	s := it.st
	if s != nil && !s.Deleted && s.Size == it.local.size && s.Mtime == it.local.mtime {
		return false, nil, nil
	}
	data, err := os.ReadFile(it.local.abs)
	if err != nil {
		return false, nil, err
	}
	if s != nil && !s.Deleted && s.Hash == crypt.SHA256Hex(data) {
		if !e.dry {
			e.mu.Lock()
			s.Size, s.Mtime = it.local.size, it.local.mtime
			e.mu.Unlock()
		}
		return false, data, nil
	}
	return true, data, nil
}

func (e *Engine) setState(key string, fs *state.FileState) {
	e.mu.Lock()
	e.st.Files[key] = fs
	e.mu.Unlock()
}

// recordLocal 记下"本机现在是 data、对应云端 etag 这个版本"。
// etag 为空（有的存储 Get 拿不到 etag，OSS SDK v2 的 GetObject 实测就返回空）时用 List 的兜底，
// 否则下轮永远觉得"远端变了"，同一批文件反复下载。
func (e *Engine) recordLocal(it *item, data []byte, etag string) {
	if etag == "" && it.remote != nil {
		etag = it.remote.ETag
	}
	e.setState(it.key, &state.FileState{ETag: etag, Size: it.local.size, Mtime: it.local.mtime, Hash: crypt.SHA256Hex(data)})
}

func (e *Engine) count(n *int) {
	e.mu.Lock()
	*n++
	e.mu.Unlock()
}

// changeRank 同一个文件一轮里可能先合并再上传，明细只留最重要的那种。
var changeRank = map[string]int{state.ChangePush: 1, state.ChangePull: 2, state.ChangeMerge: 3, state.ChangeConflict: 4, state.ChangeFork: 5}

// note 记一条本轮明细。
func (e *Engine) note(it *item, kind string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if old, ok := e.changes[it.key]; ok && changeRank[old.Kind] >= changeRank[kind] {
		return
	}
	e.changes[it.key] = state.Change{Kind: kind, PID: it.b.PID, Dir: it.b.Dir, Rel: it.rel}
}

// sortedChanges 本轮明细：重要的排前面，最多 state.MaxChanges 条。
func (e *Engine) sortedChanges() []state.Change {
	out := make([]state.Change, 0, len(e.changes))
	for _, c := range e.changes {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if a, b := changeRank[out[i].Kind], changeRank[out[j].Kind]; a != b {
			return a > b
		}
		if out[i].Dir != out[j].Dir {
			return out[i].Dir < out[j].Dir
		}
		return out[i].Rel < out[j].Rel
	})
	if len(out) > state.MaxChanges {
		out = out[:state.MaxChanges]
	}
	return out
}

// setTotal / step 进度：比对前报总数，每处理完一个文件报一次。
func (e *Engine) setTotal(n int) {
	e.mu.Lock()
	e.total += n
	done, total, fn := e.done, e.total, e.progress
	e.mu.Unlock()
	if fn != nil {
		fn(done, total)
	}
}

func (e *Engine) step() {
	e.mu.Lock()
	e.done++
	done, total, fn := e.done, e.total, e.progress
	e.mu.Unlock()
	if fn != nil {
		fn(done, total)
	}
}

// putVerified 写入后立即确认"云端现在是我写的这份"。
// 返回 ok=false 表示写完后已被别的设备覆盖——调用方不应更新本地状态。
//
// 确认方式：先问一下云端这个文件的 etag（HEAD 请求，不下载内容），和写入时返回的 etag 对得上就行；
// 存储拿不到 etag 时才退回老办法——整份读回来比对内容（长会话几十 MB，很费流量）。
// 残余风险：确认之后对方才写进来（窗口毫秒级），这种极端情况靠下一轮同步的
// "两边都变了 → 合并"兜底，最多退化成分叉，不会丢数据。
func (e *Engine) putVerified(ctx context.Context, key string, enc []byte) (etag string, ok bool, err error) {
	etag, err = e.store.Put(ctx, key, enc)
	if err != nil {
		return "", false, err
	}
	if etag != "" {
		if o, err := e.store.Stat(ctx, key); err == nil && o.ETag != "" {
			return etag, o.ETag == etag, nil
		}
	}
	got, _, err := e.store.Get(ctx, key)
	if err != nil {
		return "", false, err
	}
	return etag, bytes.Equal(got, enc), nil
}

func (e *Engine) push(ctx context.Context, it *item, data []byte) error {
	if e.dry {
		e.log.Printf("将上传 %s/%s", it.b.Dir, it.rel)
		e.count(&e.stats.Pushed)
		return nil
	}
	if data == nil {
		var err error
		if data, err = os.ReadFile(it.local.abs); err != nil {
			return err
		}
	}
	upload := data
	if isSession(it.rel) {
		upload = trimPartialLine(data)
	}
	enc, err := crypt.Seal(e.key, it.key, upload)
	if err != nil {
		return err
	}
	etag, ok, err := e.putVerified(ctx, it.key, enc)
	if err != nil {
		return err
	}
	if !ok {
		// 写入的瞬间另一台设备也写了同一个文件（我们的内容被覆盖）。
		// 不更新状态：本地内容还在磁盘上，下次同步会发现"两边都变了"并走合并，内容不丢。
		e.log.Printf("云端 %s/%s 被其他设备并发覆盖，本次保留本地，下次同步时合并", it.b.Dir, it.rel)
		return nil
	}
	// 记录的是读取前的文件大小/时间：读取后文件若又变了，下次会再传一次
	e.setState(it.key, &state.FileState{ETag: etag, Size: it.local.size, Mtime: it.local.mtime, Hash: crypt.SHA256Hex(data)})
	e.count(&e.stats.Pushed)
	e.note(it, state.ChangePush)
	return nil
}

func (e *Engine) fetchRemote(ctx context.Context, it *item) ([]byte, string, error) {
	data, etag, err := e.store.Get(ctx, it.key)
	if err != nil {
		return nil, "", err
	}
	plain, err := e.open(it.key, data)
	if err != nil {
		return nil, "", err
	}
	if isJSONish(it.rel) {
		plain = e.translatorFor(it.b).apply(plain)
	}
	return plain, etag, nil
}

func (e *Engine) localPath(it *item) string {
	if it.local != nil {
		return it.local.abs
	}
	return filepath.Join(e.cfg.ProjectsDir(), it.b.Dir, filepath.FromSlash(it.rel))
}

// writeLocal 整份写入本地文件并刷新 it.local。
// 写之前确认文件还是比对时的样子（本来没有的还没有；本来有的大小、修改时间没变），
// 否则返回 errLocalChanged：Claude Code 刚写过，这轮不覆盖。
func (e *Engine) writeLocal(it *item, data []byte) error {
	abs := e.localPath(it)
	fi, err := os.Stat(abs)
	switch {
	case it.local == nil && err == nil:
		return errLocalChanged // 本来没有，现在有了
	case it.local == nil && !errors.Is(err, fs.ErrNotExist):
		return err
	case it.local != nil && errors.Is(err, fs.ErrNotExist):
		return errLocalChanged // 刚被删了
	case it.local != nil && err != nil:
		return err
	case it.local != nil && (fi.Size() != it.local.size || fi.ModTime().UnixNano() != it.local.mtime):
		return errLocalChanged
	}
	if err := fsutil.WriteFileAtomic(abs, data); err != nil {
		return err
	}
	if fi, err = os.Stat(abs); err != nil {
		return err
	}
	it.local = &localFile{abs: abs, size: fi.Size(), mtime: fi.ModTime().UnixNano()}
	return nil
}

// appendLocal 把缺的行追加到本地文件末尾（只追加、不重写），返回追加后的完整内容，并刷新 it.local。
// 追加期间 Claude Code 又写了（文件比我们算的长）：it.local 的修改时间记成 0，下轮一定重新读、重新比对。
func (e *Engine) appendLocal(it *item, local []byte, lines [][]byte) ([]byte, error) {
	abs := e.localPath(it)
	merged, err := appendLines(abs, local, lines)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	it.local = &localFile{abs: abs, size: fi.Size(), mtime: fi.ModTime().UnixNano()}
	if fi.Size() != int64(len(merged)) {
		it.local.mtime = 0
	}
	return merged, nil
}

// pull 用云端版本整份替换本机文件（会话不走这里，见 syncItem / merge）。本机原来有的话，旧版本先进备份。
func (e *Engine) pull(ctx context.Context, it *item, data []byte, etag string) error {
	if e.dry {
		e.log.Printf("将下载 %s/%s", it.b.Dir, it.rel)
		e.count(&e.stats.Pulled)
		return nil
	}
	if data == nil {
		var err error
		if data, etag, err = e.fetchRemote(ctx, it); err != nil {
			return err
		}
	}
	if it.local != nil {
		cur, err := os.ReadFile(it.local.abs)
		if err != nil {
			return err
		}
		if bytes.Equal(cur, data) {
			// 内容其实一样（比如改口令后云端重新加密过），只更新状态
			e.recordLocal(it, data, etag)
			return nil
		}
		e.bk.save(BackupLocal, it.b.PID, it.b.Dir, it.rel, cur)
	}
	if err := e.writeLocal(it, data); err != nil {
		return err
	}
	e.recordLocal(it, data, etag)
	e.count(&e.stats.Pulled)
	e.note(it, state.ChangePull)
	return nil
}

// merge 处理两边都改过（或第一次见面、内容不同）的文件，以及云端变了的会话。
func (e *Engine) merge(ctx context.Context, it *item, local []byte) error {
	if e.dry {
		e.log.Printf("将比对合并 %s/%s", it.b.Dir, it.rel)
		e.count(&e.stats.Merged)
		return nil
	}
	var err error
	if local == nil {
		if local, err = os.ReadFile(it.local.abs); err != nil {
			return err
		}
	}
	remote, etag, err := e.fetchRemote(ctx, it)
	if err != nil {
		return err
	}
	if bytes.Equal(local, remote) {
		e.recordLocal(it, local, etag)
		return nil
	}

	if !isSession(it.rel) {
		if filepath.Base(it.rel) == "MEMORY.md" {
			// 记忆索引每行一条，取并集：只把另一台独有的行追加到末尾
			merged, err := e.appendLocal(it, local, missingLines(local, remote, textLineKey))
			if err != nil {
				return err
			}
			e.count(&e.stats.Merged)
			e.note(it, state.ChangeMerge)
			return e.push(ctx, it, merged)
		}
		// 其他文件：本机优先，云端那份进备份。
		// 不能靠比较 mtime 决胜负：本机时钟和云端服务器时钟不可比（时钟偏移会让旧的赢）。
		// 本机这份一定是本机用户最近的真实编辑；被覆盖的远端版本留在 backup/ 里。
		e.bk.save(BackupRemote, it.b.PID, it.b.Dir, it.rel, remote)
		e.note(it, state.ChangeConflict)
		return e.push(ctx, it, local)
	}

	// 最后一行没写完：Claude Code 正写到一半，这轮先不动（很久没动的半行是写崩了留下的，不再等它）
	if !lastLineComplete(local) && time.Since(time.Unix(0, it.local.mtime)) < time.Minute {
		return errLocalChanged
	}
	cmp := trimPartialLine(local)
	lk, rk := keySet(cmp), keySet(remote)
	switch {
	case isSubset(rk, lk):
		return e.push(ctx, it, local)
	case isSubset(lk, rk):
		// 另一台设备接着聊了几句：把多出来的记录追加到本机末尾
		merged, err := e.appendLocal(it, local, missingLines(cmp, remote, recordLineKey))
		if err != nil {
			return err
		}
		e.recordLocal(it, merged, etag)
		e.count(&e.stats.Pulled)
		e.note(it, state.ChangePull)
		return nil
	}

	if id, ok := isTopLevelSession(it.rel); ok {
		// 分叉 ID 由"旧会话 ID + 远端内容"确定：推完 fork、推主文件失败时，
		// 重试会得到同一个 ID（幂等），不会每失败一次多一份重复会话。
		newID := forkUUID(id, remote)
		fork := &item{key: FileKey(it.b.PID, newID+".jsonl"), rel: newID + ".jsonl", b: it.b}
		if _, err := os.Stat(e.localPath(fork)); errors.Is(err, fs.ErrNotExist) {
			forked := forkSession(remote, id, newID)
			if err := e.writeLocal(fork, forked); err != nil {
				return err
			}
			if err := e.push(ctx, fork, forked); err != nil {
				return err
			}
			e.bk.fork(it.b.PID, it.b.Dir, it.rel, fork.rel)
			e.mu.Lock()
			e.st.Forks[newID] = id
			e.mu.Unlock()
			e.log.Printf("会话 %s 在两台设备上各自继续了，另一份已另存为新会话 %s", id, newID)
			e.count(&e.stats.Forked)
			e.note(fork, state.ChangeFork)
		} // 已经有了：上次分叉时写好了、推主文件失败了，这次不重复写（它自己作为一个文件照常同步）
		return e.push(ctx, it, local)
	}
	// 子代理等内部记录：按 uuid 取并集，另一台独有的记录追加到末尾
	merged, err := e.appendLocal(it, local, missingLines(cmp, remote, lineKey))
	if err != nil {
		return err
	}
	e.count(&e.stats.Merged)
	e.note(it, state.ChangeMerge)
	return e.push(ctx, it, merged)
}
