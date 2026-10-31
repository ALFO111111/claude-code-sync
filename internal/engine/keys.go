package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ALFO111111/agent-sync/internal/config"
	"github.com/ALFO111111/agent-sync/internal/crypt"
	"github.com/ALFO111111/agent-sync/internal/i18n"
	"github.com/ALFO111111/agent-sync/internal/logging"
	"github.com/ALFO111111/agent-sync/internal/state"
	"github.com/ALFO111111/agent-sync/internal/storage"
)

// 口令（密钥）：
//
//	keycheck       云端的口令校验文件。能用本机密钥解开 = 本机口令和云端一致
//	每轮同步开始   先看一眼 keycheck（HEAD，没变就不下载）：解不开说明别的设备改了口令，本机停下来等用户输新口令，
//	               免得接着用旧密钥上传，把新旧密钥的文件混在一起
//	修改口令       全部云端文件用新密钥重新加密 → 最后换 keycheck → 换本机密钥（旧的留作 key.prev）
//	key.prev       改口令前的旧密钥：别的设备还没换口令时传上来的文件，用它也能解开，之后再传就是新密钥了

// KeycheckKey 口令校验文件在云端的 key。
const KeycheckKey = "keycheck"

// ErrKeyChanged 云端口令校验文件解不开：别的设备改了口令。
var ErrKeyChanged = i18n.Err("同步口令已在其他设备上修改，请在本机输入新口令后再同步")

// ErrWrongPassphrase 输入的口令不对。
var ErrWrongPassphrase = i18n.Err("口令不对")

// CheckKey 能否用 key 解开口令校验文件。
func CheckKey(key, keycheck []byte) bool {
	plain, err := crypt.Open(key, KeycheckKey, keycheck)
	return err == nil && string(plain) == crypt.KeycheckPlain
}

// SealKeycheck 用 key 生成口令校验文件。
func SealKeycheck(key []byte) ([]byte, error) {
	return crypt.Seal(key, KeycheckKey, []byte(crypt.KeycheckPlain))
}

// open 解密云端文件：先用当前密钥，不行再试改口令前的旧密钥。
func (e *Engine) open(aad string, data []byte) ([]byte, error) {
	plain, err := crypt.Open(e.key, aad, data)
	if err != nil && e.prevKey != nil {
		if p, perr := crypt.Open(e.prevKey, aad, data); perr == nil {
			return p, nil
		}
	}
	return plain, err
}

// verifyKey 确认本机密钥能解开云端的口令校验文件。校验文件没变（etag 一样）就不下载。
func (e *Engine) verifyKey(ctx context.Context) error {
	o, err := e.store.Stat(ctx, KeycheckKey)
	if errors.Is(err, storage.ErrNotFound) {
		return nil // 还没有校验文件（测试用的本地存储）
	}
	if err != nil {
		return fmt.Errorf(i18n.T("读取云端口令校验文件失败: %w"), err)
	}
	if o.ETag != "" && o.ETag == e.st.KeycheckETag && !e.st.KeyMismatch {
		return nil
	}
	data, _, err := e.store.Get(ctx, KeycheckKey)
	if err != nil {
		return fmt.Errorf(i18n.T("读取云端口令校验文件失败: %w"), err)
	}
	if !CheckKey(e.key, data) {
		if !e.st.KeyMismatch {
			e.log.Printf("云端的同步口令已在其他设备上修改，本机暂停同步，等待输入新口令")
		}
		e.st.KeyMismatch = true
		return ErrKeyChanged
	}
	e.st.KeycheckETag, e.st.KeyMismatch = o.ETag, false
	return nil
}

func openStore() (*config.Config, storage.Storage, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	store, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, nil, err
	}
	return cfg, store, nil
}

// ChangeKey 修改口令：云端全部文件用 newKey 重新加密，最后换口令校验文件和本机密钥。
// oldKey 必须是本机现在的密钥。中途失败可以原样重来：已经是新密钥的文件会跳过。
// 期间持有同步锁（本机不会同时同步）；progress 可为 nil。
func ChangeKey(ctx context.Context, oldKey, newKey []byte, progress func(done, total int)) error {
	if cur, err := config.LoadKey(); err != nil {
		return err
	} else if !bytes.Equal(cur, oldKey) {
		return ErrWrongPassphrase
	}
	if bytes.Equal(oldKey, newKey) {
		return errors.New(i18n.T("新口令和原来的一样"))
	}
	_, store, err := openStore()
	if err != nil {
		return err
	}
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	log := logging.New(true)

	// 云端的口令校验必须是旧口令（或者上次改到一半，已经是新口令了）：
	// 都不是说明别的设备改过口令，本机得先换上那个口令才能改
	if kc, _, err := store.Get(ctx, KeycheckKey); err == nil {
		if !CheckKey(oldKey, kc) && !CheckKey(newKey, kc) {
			return ErrKeyChanged
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	objs, err := store.List(ctx, "")
	if err != nil {
		return fmt.Errorf(i18n.T("列出云端文件失败: %w"), err)
	}
	var work []storage.Object
	for _, o := range objs {
		if o.Key != KeycheckKey {
			work = append(work, o)
		}
	}
	prevKey := config.LoadPrevKey()

	var mu sync.Mutex
	done, failed, unreadable := 0, 0, 0
	newTags := map[string][2]string{} // key → {旧 etag, 新 etag}
	if progress != nil {
		progress(0, len(work))
	}
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, o := range work {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			etag, err := rekeyObject(ctx, store, o.Key, oldKey, prevKey, newKey)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case errors.Is(err, errUnreadable):
				unreadable++ // 本来就解不开（损坏或别的口令），原样留着
				log.Printf("改口令：%s 用原口令也解不开，跳过", o.Key)
			case err != nil:
				failed++
				log.Printf("改口令：%s 重新加密失败: %v", o.Key, err)
			case etag != "":
				newTags[o.Key] = [2]string{o.ETag, etag}
			}
			done++
			if progress != nil {
				progress(done, len(work))
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf(i18n.T("有 %d 个文件没能重新加密（网络问题？），口令还没改成功，请再试一次"), failed)
	}

	// 口令校验文件最后换：其他设备看到它变了才会停下来，等用户输入新口令
	kc, err := SealKeycheck(newKey)
	if err != nil {
		return err
	}
	kcTag, err := store.Put(ctx, KeycheckKey, kc)
	if err != nil {
		return err
	}
	if err := config.ReplaceKey(newKey); err != nil {
		return err
	}

	// 本机状态里的 etag 跟着换：内容没变，只是重新加密了，不然下轮会把所有文件当成"云端变了"重新比对一遍
	st, err := state.Load()
	if err != nil {
		return err
	}
	for k, fs := range st.Files {
		if t, ok := newTags[k]; ok && fs.ETag == t[0] {
			fs.ETag = t[1]
		}
	}
	if t, ok := newTags[HistoryKey]; ok && st.History != nil && st.History.ETag == t[0] {
		st.History.ETag = t[1]
	}
	for id, d := range st.Devices {
		if t, ok := newTags["devices/"+id+".json"]; ok && d.ETag == t[0] {
			d.ETag = t[1]
			st.Devices[id] = d
		}
	}
	st.KeycheckETag, st.KeyMismatch = kcTag, false
	if err := state.Save(st); err != nil {
		return err
	}
	log.Printf("同步口令已修改：%d 个云端文件已用新口令重新加密", len(newTags))
	if unreadable > 0 {
		log.Printf("有 %d 个云端文件原来就解不开，没有处理", unreadable)
	}
	return nil
}

var errUnreadable = errors.New("unreadable")

// rekeyObject 把一个云端文件换成新密钥加密，返回新 etag；已经是新密钥的返回空 etag。网络错误重试 3 次。
func rekeyObject(ctx context.Context, store storage.Storage, key string, oldKey, prevKey, newKey []byte) (string, error) {
	var data []byte
	err := retry(ctx, func() (err error) { data, _, err = store.Get(ctx, key); return })
	if err != nil {
		return "", err
	}
	if _, err := crypt.Open(newKey, key, data); err == nil {
		return "", nil // 上次改到一半时已经换过了
	}
	plain, err := crypt.Open(oldKey, key, data)
	if err != nil && prevKey != nil {
		plain, err = crypt.Open(prevKey, key, data)
	}
	if err != nil {
		return "", errUnreadable
	}
	enc, err := crypt.Seal(newKey, key, plain)
	if err != nil {
		return "", err
	}
	var etag string
	err = retry(ctx, func() (err error) { etag, err = store.Put(ctx, key, enc); return })
	return etag, err
}

func retry(ctx context.Context, fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = fn(); err == nil || ctx.Err() != nil {
			return err
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return err
}

// AdoptKey 别的设备改过口令后，本机换上新口令（先用云端的口令校验文件确认新口令是对的）。
func AdoptKey(ctx context.Context, newKey []byte) error {
	_, store, err := openStore()
	if err != nil {
		return err
	}
	kc, _, err := store.Get(ctx, KeycheckKey)
	if err != nil {
		return fmt.Errorf(i18n.T("读取云端口令校验文件失败: %w"), err)
	}
	if !CheckKey(newKey, kc) {
		return ErrWrongPassphrase
	}
	unlock, err := state.WaitLock(30 * time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	if cur, err := config.LoadKey(); err != nil || !bytes.Equal(cur, newKey) {
		if err := config.ReplaceKey(newKey); err != nil {
			return err
		}
	}
	st, err := state.Load()
	if err != nil {
		return err
	}
	st.KeyMismatch, st.KeycheckETag = false, ""
	if err := state.Save(st); err != nil {
		return err
	}
	logging.New(true).Printf("已换上新的同步口令")
	return nil
}

// KeyMatchesCloud 本机密钥能不能解开云端的口令校验文件（云端还没有校验文件也算能）。
func KeyMatchesCloud(ctx context.Context) (bool, error) {
	_, store, err := openStore()
	if err != nil {
		return false, err
	}
	key, err := config.LoadKey()
	if err != nil {
		return false, err
	}
	kc, _, err := store.Get(ctx, KeycheckKey)
	if errors.Is(err, storage.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf(i18n.T("读取云端口令校验文件失败: %w"), err)
	}
	return CheckKey(key, kc), nil
}
