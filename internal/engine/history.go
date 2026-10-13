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

	"github.com/ALFO111111/claude-code-sync/internal/crypt"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/state"
	"github.com/ALFO111111/claude-code-sync/internal/storage"
)

// 输入历史 ~/.claude/history.jsonl 的同步（Claude Code 命令行里按 ↑ 翻到的就是它）。
//
// 每行一条：{"display":"你输入的话","timestamp":毫秒,"project":"本机项目路径","sessionId":...}
// 它是全机一份、所有项目混在一起，所以不走项目文件那套逻辑，单独处理：
//
//	云端    history.jsonl 一份，内容是最后上传它的那台设备的本地文件原样
//	同一条  按 (timestamp, display) 判断，和路径无关（翻译前后都能认出是同一条）
//	拉下来  project 字段翻译成本机路径，然后只往本地文件末尾追加缺的行，
//	        绝不重写已有内容：Claude Code 随时可能在追加，重写整个文件可能吞掉它刚写的那行
//	推上去  本地有云端没有的行时，把本地文件整份传上去
//
// 只增不减：在一台电脑上删掉的历史，会被另一台同步回来。

// HistoryKey 输入历史在云端的 key。
const HistoryKey = "history.jsonl"

func (e *Engine) historyPath() string { return filepath.Join(e.cfg.ClaudeDir, "history.jsonl") }

func (e *Engine) syncHistory(ctx context.Context) error {
	objs, err := e.store.List(ctx, HistoryKey)
	if err != nil {
		return fmt.Errorf(i18n.T("列出云端输入历史失败: %w"), err)
	}
	var remote *storage.Object
	for i := range objs {
		if objs[i].Key == HistoryKey {
			remote = &objs[i]
		}
	}
	path := e.historyPath()
	fi, err := os.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if fi == nil && remote == nil {
		return nil
	}
	s := e.st.History
	if s != nil && fi != nil && remote != nil &&
		s.ETag == remote.ETag && s.Size == fi.Size() && s.Mtime == fi.ModTime().UnixNano() {
		return nil // 两边都没变
	}

	var local []byte
	if fi != nil {
		if local, err = os.ReadFile(path); err != nil {
			return err
		}
	}
	var remoteLines [][]byte
	if remote != nil {
		data, _, err := e.store.Get(ctx, HistoryKey)
		if err != nil {
			return err
		}
		plain, err := crypt.Open(e.key, HistoryKey, data)
		if err != nil {
			return err
		}
		roots := e.historyRoots()
		for _, l := range SplitLines(plain) {
			remoteLines = append(remoteLines, translateHistoryLine(bytes.TrimRight(l, "\r"), roots))
		}
	}

	localKeys := map[string]bool{}
	for _, l := range SplitLines(local) {
		localKeys[historyLineKey(l)] = true
	}
	remoteKeys := map[string]bool{}
	var missing [][]byte // 云端有、本地没有的行
	for _, l := range remoteLines {
		k := historyLineKey(l)
		if !remoteKeys[k] && !localKeys[k] {
			missing = append(missing, l)
		}
		remoteKeys[k] = true
	}
	needPush := remote == nil
	for k := range localKeys {
		if !remoteKeys[k] {
			needPush = true
			break
		}
	}

	if e.dry {
		if len(missing) > 0 || needPush {
			e.log.Printf("将同步输入历史：拉下 %d 条，本地有新内容=%v", len(missing), needPush)
		}
		return nil
	}

	if len(missing) > 0 {
		if !lastLineComplete(local) {
			// Claude Code 正写到一半，这轮先不动，下次再拉
			e.log.Printf("输入历史正在被写入，本次跳过")
			return nil
		}
		if local, err = appendHistory(path, local, missing); err != nil {
			return err
		}
		e.mu.Lock()
		e.stats.History += len(missing)
		e.mu.Unlock()
		e.log.Printf("输入历史：从其他设备补进 %d 条", len(missing))
	}

	etag := ""
	if remote != nil {
		etag = remote.ETag
	}
	if needPush {
		upload := trimPartialLine(local)
		enc, err := crypt.Seal(e.key, HistoryKey, upload)
		if err != nil {
			return err
		}
		newTag, ok, err := e.putVerified(ctx, HistoryKey, enc)
		if err != nil {
			return err
		}
		if !ok {
			// 被别的设备同时覆盖了：不记状态，下轮重新比对合并
			e.st.History = nil
			return nil
		}
		etag = newTag
	}

	fi, err = os.Stat(path)
	if err != nil {
		return err
	}
	st := &state.FileState{ETag: etag, Size: fi.Size(), Mtime: fi.ModTime().UnixNano()}
	if fi.Size() != int64(len(local)) {
		st.Mtime = 0 // 刚才这段时间 Claude Code 又写了：记成"变过"，下轮再比对
	}
	e.st.History = st
	return nil
}

// appendHistory 把缺的行追加到本地文件末尾（只追加、不重写），返回追加后的完整内容。
func appendHistory(path string, local []byte, lines [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	if len(local) > 0 && local[len(local)-1] != '\n' {
		buf.WriteByte('\n') // 最后一行是完整的但没换行
	}
	for _, l := range lines {
		buf.Write(l)
		buf.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	_, werr := f.Write(buf.Bytes())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return nil, werr
	}
	return append(local, buf.Bytes()...), nil
}

// lastLineComplete 文件为空、以换行结尾、或最后一行是完整的 JSON。
func lastLineComplete(data []byte) bool {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return true
	}
	return json.Valid(data[bytes.LastIndexByte(data, '\n')+1:])
}

// historyLineKey 判断两行"是不是同一条"：时间戳 + 输入内容，不看路径。解析失败的行按原文比。
func historyLineKey(line []byte) string {
	var v struct {
		Timestamp json.RawMessage `json:"timestamp"`
		Display   string          `json:"display"`
	}
	line = bytes.TrimSpace(line)
	if json.Unmarshal(line, &v) == nil && len(v.Timestamp) > 0 {
		return string(v.Timestamp) + "\x00" + v.Display
	}
	return "raw\x00" + string(line)
}

// rootMap 一条路径对照：其他设备上的项目根路径 → 本机路径。
type rootMap struct{ from, to string }

// historyRoots 所有绑定项目的路径对照，最长的排前面：
// 用户目录 C:\Users\x 是很多项目的上级，必须让 C:\Users\x\proj 先匹配。
func (e *Engine) historyRoots() []rootMap {
	var out []rootMap
	for _, b := range e.st.Bindings {
		for _, f := range e.ForeignRoots(b.PID) {
			if f != "" && !fsutil.SamePath(f, b.Root) {
				out = append(out, rootMap{from: strings.TrimRight(f, `\/`), to: strings.TrimRight(b.Root, `\/`)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].from) > len(out[j].from) })
	return out
}

// translateRoot 路径是某个对方根路径本身或在它下面，就换成本机路径；都不是就原样返回。
func translateRoot(p string, roots []rootMap) string {
	slash := func(s string) string { return strings.ReplaceAll(s, "/", `\`) }
	for _, r := range roots {
		n := len(r.from)
		if len(p) < n || !strings.EqualFold(slash(p[:n]), slash(r.from)) {
			continue
		}
		switch rest := p[n:]; {
		case rest == "":
			return r.to
		case rest[0] == '\\' || rest[0] == '/':
			return r.to + slash(rest)
		}
	}
	return p
}

// translateHistoryLine 只改 project 字段；没变化或解析不了就原样返回。
func translateHistoryLine(line []byte, roots []rootMap) []byte {
	if len(roots) == 0 {
		return line
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(line, &m) != nil {
		return line
	}
	var p string
	if json.Unmarshal(m["project"], &p) != nil || p == "" {
		return line
	}
	np := translateRoot(p, roots)
	if np == p {
		return line
	}
	m["project"], _ = json.Marshal(np)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if enc.Encode(m) != nil {
		return line
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}
