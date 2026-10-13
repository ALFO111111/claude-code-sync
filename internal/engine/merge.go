package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/crypt"
)

// 会话文件（.jsonl）每行一条记录，带 uuid / parentUuid，本质是一棵只追加的树。
// 合并规则：
//   一边的记录是另一边的子集  → 取多的那份（绝大多数情况：另一台设备接着聊了几句）
//   两边各有对方没有的记录    → 顶层会话另存为新会话（两份都保留）；子代理等内部文件按记录取并集

// SplitLines 把 jsonl 按行切开，跳过空行。
func SplitLines(data []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

func lineKey(line []byte) string {
	var v struct {
		UUID      string `json:"uuid"`
		MessageID string `json:"messageId"`
		Type      string `json:"type"`
	}
	if json.Unmarshal(line, &v) == nil {
		if v.UUID != "" {
			return "u:" + v.UUID
		}
		if v.MessageID != "" {
			return "m:" + v.Type + ":" + v.MessageID
		}
	}
	return "h:" + crypt.SHA256Hex(bytes.TrimSpace(line))
}

func keySet(data []byte) map[string]bool {
	set := map[string]bool{}
	for _, l := range SplitLines(data) {
		set[lineKey(l)] = true
	}
	return set
}

func isSubset(a, b map[string]bool) bool {
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// unionLines 保留本地全部记录，再追加远端独有的记录。
func unionLines(local, remote []byte) []byte {
	return unionBy(local, remote, lineKey)
}

// unionTextLines 文本文件按行取并集（MEMORY.md 用），key 是去掉首尾空白的整行。
func unionTextLines(local, remote []byte) []byte {
	return unionBy(local, remote, func(l []byte) string { return strings.TrimSpace(string(l)) })
}

// unionBy 保留本地全部行，再追加远端独有的行（key 决定"算不算同一行"）。
// 行尾的 \r 统一剥掉（CRLF 文件混进 \r 会让 jsonl 解析和文本比对变脏）。
func unionBy(local, remote []byte, key func([]byte) string) []byte {
	have := map[string]bool{}
	var out []byte
	for _, data := range [][]byte{local, remote} {
		for _, l := range SplitLines(data) {
			l = bytes.TrimRight(l, "\r")
			k := key(l)
			if have[k] {
				continue
			}
			have[k] = true
			out = append(out, l...)
			out = append(out, '\n')
		}
	}
	return out
}

// forkUUID 由旧会话 ID + 远端内容确定性地算出新会话 ID。
// 这样"fork 推上去、主文件推失败"的重试会得到同一个 ID（幂等），
// 不会每失败一次就多一份内容相同的重复会话。
func forkUUID(sessionID string, remote []byte) string {
	sum := sha256.Sum256(append([]byte("github.com/ALFO111111/claude-code-sync/fork\x00"+sessionID+"\x00"), remote...))
	return uuidFromBytes(sum[:16])
}

// forkSession 把一份会话改成新的会话 ID，作为独立会话保存。
func forkSession(data []byte, oldID, newID string) []byte {
	return bytes.ReplaceAll(data, []byte(`"sessionId":"`+oldID+`"`), []byte(`"sessionId":"`+newID+`"`))
}

// trimPartialLine 去掉末尾还没写完的半行（Claude 正在写入时可能读到），下次同步再传完整的。
func trimPartialLine(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	i := bytes.LastIndexByte(data, '\n')
	if json.Valid(data[i+1:]) {
		return data
	}
	return data[:i+1]
}

// isTopLevelSession 判断是不是项目目录下直接的会话文件（<uuid>.jsonl）。
func isTopLevelSession(rel string) (sessionID string, ok bool) {
	if strings.Contains(rel, "/") || !strings.HasSuffix(rel, ".jsonl") {
		return "", false
	}
	id := strings.TrimSuffix(rel, ".jsonl")
	return id, len(id) == 36 && strings.Count(id, "-") == 4
}

// uuidFromBytes 把 16 字节格式化成 UUID（置 v4/variant 位）。
func uuidFromBytes(b []byte) string {
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
