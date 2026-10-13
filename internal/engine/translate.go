package engine

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
)

// 路径翻译：云端存的是原样内容，拉到本机时，把"其他设备上的项目根路径"换成"本机的项目根路径"。
//
// 只替换 JSON 字符串里"整个值以项目根路径开头"的情况（如 cwd、file_path 字段），
// 聊天正文中间夹带的路径不动，避免改坏代码和 diff。因为根路径是绑定时精确知道的，
// 不需要猜"路径到哪里结束"，所以中文、空格、@、括号都不会出问题。

type translator struct {
	res   []*regexp.Regexp
	local string
}

func newTranslator(foreign []string, local string) *translator {
	seen := map[string]bool{}
	var roots []string
	for _, r := range foreign {
		r = strings.TrimRight(r, `\/`)
		k := strings.ToLower(strings.ReplaceAll(r, "/", `\`))
		if r == "" || seen[k] || fsutil.SamePath(r, local) {
			continue
		}
		seen[k] = true
		roots = append(roots, r)
	}
	if len(roots) == 0 {
		return nil
	}
	// 长的先换，避免 D:\a 抢先匹配 D:\a\b
	sort.Slice(roots, func(i, j int) bool { return len(roots[i]) > len(roots[j]) })
	t := &translator{local: strings.TrimRight(local, `\/`)}
	for _, r := range roots {
		t.res = append(t.res, regexp.MustCompile(`(?i)"`+rootPattern(r)+`(\\\\|/|")`))
	}
	return t
}

// rootPattern 生成匹配 JSON 文本中某个路径的正则：分隔符既可以是 \\（JSON 转义后的反斜杠）也可以是 /。
func rootPattern(root string) string {
	var b strings.Builder
	for _, r := range root {
		if r == '\\' || r == '/' {
			b.WriteString(`(?:\\\\|/)`)
		} else {
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	return b.String()
}

func (t *translator) apply(data []byte) []byte {
	if t == nil {
		return data
	}
	backslash := []byte(`"` + strings.ReplaceAll(t.local, `\`, `\\`))
	slash := []byte(`"` + strings.ReplaceAll(t.local, `\`, `/`))
	for _, re := range t.res {
		data = re.ReplaceAllFunc(data, func(m []byte) []byte {
			var boundary []byte
			switch {
			case bytes.HasSuffix(m, []byte(`\\`)):
				boundary = m[len(m)-2:]
			default:
				boundary = m[len(m)-1:]
			}
			head := m[:len(m)-len(boundary)]
			out := slash
			if bytes.Contains(head, []byte(`\\`)) || bytes.Equal(boundary, []byte(`\\`)) {
				out = backslash
			}
			return append(append([]byte{}, out...), boundary...)
		})
	}
	return data
}

func isJSONish(rel string) bool {
	l := strings.ToLower(rel)
	return strings.HasSuffix(l, ".jsonl") || strings.HasSuffix(l, ".json")
}
