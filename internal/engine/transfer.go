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

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/project"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// 文件同步：逐个比对本地和云端文件，决定上传、下载、合并还是分叉；以及冲突备份。

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
			defer func() { <-sem; wg.Done() }()
			if err := e.syncItem(ctx, it); err != nil {
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
			return e.pull(ctx, it, nil, "")
		default:
			return e.merge(ctx, it, data)
		}
	}
	return nil
}

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

func (e *Engine) count(n *int) {
	e.mu.Lock()
	*n++
	e.mu.Unlock()
}

// putVerified 写入后立即读回校验 etag（OSS 是强一致的）。
// 返回 ok=false 表示写完后内容已被别的设备覆盖——调用方不应更新本地状态。
// 残余风险：校验之后对方才写进来（窗口毫秒级），这种极端情况靠下一轮同步的
// "两边都变了 → 合并"兜底，最多退化成分叉，不会丢数据。
func (e *Engine) putVerified(ctx context.Context, key string, enc []byte) (etag string, ok bool, err error) {
	etag, err = e.store.Put(ctx, key, enc)
	if err != nil {
		return "", false, err
	}
	// 写后读校验：直接比对内容，不比 etag——有的存储 Get 拿不到 etag
	// （OSS SDK v2 的 GetObject 实测就返回空），比内容才是最准的"我的写入有没有被抢盖"。
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
	if strings.HasSuffix(it.rel, ".jsonl") {
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
	return nil
}

func (e *Engine) fetchRemote(ctx context.Context, it *item) ([]byte, string, error) {
	data, etag, err := e.store.Get(ctx, it.key)
	if err != nil {
		return nil, "", err
	}
	plain, err := crypt.Open(e.key, it.key, data)
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

// writeLocal 写入本地文件并刷新 it.local。
func (e *Engine) writeLocal(it *item, data []byte) error {
	abs := e.localPath(it)
	if err := fsutil.WriteFileAtomic(abs, data); err != nil {
		return err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return err
	}
	it.local = &localFile{abs: abs, size: fi.Size(), mtime: fi.ModTime().UnixNano()}
	return nil
}

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
	if err := e.writeLocal(it, data); err != nil {
		return err
	}
	if etag == "" && it.remote != nil {
		// 存储的 Get 拿不到 etag（OSS SDK v2 的 GetObject 返回空）时，
		// 用 List 的 etag 兜底，否则下轮永远觉得"远端变了"，同一批文件反复下载。
		etag = it.remote.ETag
	}
	e.setState(it.key, &state.FileState{ETag: etag, Size: it.local.size, Mtime: it.local.mtime, Hash: crypt.SHA256Hex(data)})
	e.count(&e.stats.Pulled)
	return nil
}

// merge 处理两边都改过（或第一次见面、内容不同）的文件。
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
		if etag == "" {
			etag = it.remote.ETag // Get 拿不到 etag 时用 List 的兜底（同 pull）
		}
		e.setState(it.key, &state.FileState{ETag: etag, Size: it.local.size, Mtime: it.local.mtime, Hash: crypt.SHA256Hex(local)})
		return nil
	}

	if !strings.HasSuffix(it.rel, ".jsonl") {
		if filepath.Base(it.rel) == "MEMORY.md" {
			// 记忆索引每行一条，取并集
			merged := unionTextLines(local, remote)
			if err := e.writeLocal(it, merged); err != nil {
				return err
			}
			e.count(&e.stats.Merged)
			return e.push(ctx, it, merged)
		}
		// 其他文件：本机优先，云端那份进备份。
		// 不能靠比较 mtime 决胜负：本机时钟和 OSS 服务器时钟不可比（时钟偏移会让旧的赢）。
		// 本机这份一定是本机用户最近的真实编辑；被覆盖的远端版本留在 backup/ 里 14 天。
		e.backup(it, remote, ".remote")
		return e.push(ctx, it, local)
	}

	lk, rk := keySet(local), keySet(remote)
	switch {
	case isSubset(rk, lk):
		return e.push(ctx, it, local)
	case isSubset(lk, rk):
		return e.pull(ctx, it, remote, etag)
	}

	if id, ok := isTopLevelSession(it.rel); ok {
		// 分叉 ID 由"旧会话 ID + 远端内容"确定：推完 fork、推主文件失败时，
		// 重试会得到同一个 ID（幂等），不会每失败一次多一份重复会话。
		newID := forkUUID(id, remote)
		fork := &item{key: FileKey(it.b.PID, newID+".jsonl"), rel: newID + ".jsonl", b: it.b}
		forked := forkSession(remote, id, newID)
		if err := e.writeLocal(fork, forked); err != nil {
			return err
		}
		if err := e.push(ctx, fork, forked); err != nil {
			return err
		}
		e.log.Printf("会话 %s 在两台设备上各自继续了，另一份已另存为新会话 %s", id, newID)
		e.count(&e.stats.Forked)
		return e.push(ctx, it, local)
	}
	merged := unionLines(local, remote)
	if err := e.writeLocal(it, merged); err != nil {
		return err
	}
	e.count(&e.stats.Merged)
	return e.push(ctx, it, merged)
}

// ---------- 备份 ----------

func (e *Engine) backup(it *item, data []byte, suffix string) {
	p := filepath.Join(config.Dir(), "backup", e.runTS, it.b.Dir, filepath.FromSlash(it.rel)+suffix)
	if err := fsutil.WriteFileAtomic(p, data); err != nil {
		e.log.Printf("备份失败 %s: %v", p, err)
	}
}

func (e *Engine) pruneBackups() {
	dir := filepath.Join(config.Dir(), "backup")
	entries, _ := os.ReadDir(dir)
	for _, ent := range entries {
		t, err := time.ParseInLocation("20060102-150405", strings.SplitN(ent.Name(), ".", 2)[0], time.Local)
		if err == nil && time.Since(t) > 14*24*time.Hour {
			os.RemoveAll(filepath.Join(dir, ent.Name()))
		}
	}
}
