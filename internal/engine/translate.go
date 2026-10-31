package engine

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/ALFO111111/agent-sync/internal/fsutil"
)

// 路径翻译：云端存的是原样内容，拉到本机时，把"其他设备上的项目根路径"换成"本机的项目根路径"。
//
// 只替换 JSON 字符串里"整个值以项目根路径开头"的情况（如 cwd、file_path 字段），
// 聊天正文中间夹带的路径不动，避免改坏代码和 diff。因为根路径是绑定时精确知道的，
// 不需要猜"路径到哪里结束"，所以中文、空格、@、括号都不会出问题。
//
// 跨系统（Windows ↔ macOS）时，根路径后面那一段的分隔符也一起换成本机的写法：
// D:\code\x\src\a.go 到 Mac 上是 /Users/me/code/x/src/a.go，而不是 /Users/me/code/x\src\a.go。
//
// Windows 上 Claude Code 用 Git Bash 跑过命令后，cwd 会记成 /d/code/x 这种写法，也一起换：
// 到 Mac 上换成 Mac 的路径，到另一台 Windows 上换成那台的 Git Bash 写法（/e/work/x）。

type translateRule struct {
	re      *regexp.Regexp
	local   string
	crossOS bool // 对方和本机是不同系统（路径风格不同）
}

type translator struct {
	rules []translateRule
}

// newTranslator 多个"其他设备上的根路径"都换成同一个本机根路径（同一项目在各设备上的位置）。
func newTranslator(foreign []string, local string) *translator {
	pairs := make([][2]string, 0, len(foreign))
	for _, f := range foreign {
		pairs = append(pairs, [2]string{f, local})
	}
	return newPairTranslator(pairs)
}

// newPairTranslator 每一对是（其他设备上的路径 → 本机路径）。长的先换，避免 D:\a 抢先匹配 D:\a\b。
func newPairTranslator(pairs [][2]string) *translator {
	seen := map[string]bool{}
	type pair struct{ foreign, local string }
	var list []pair
	for _, p := range pairs {
		f, l := strings.TrimRight(p[0], `\/`), strings.TrimRight(p[1], `\/`)
		k := strings.ToLower(strings.ReplaceAll(f, "/", `\`))
		if f == "" || l == "" || seen[k] || (fsutil.SamePath(f, l) && isWindowsPath(f) == isWindowsPath(l)) {
			continue
		}
		seen[k] = true
		list = append(list, pair{f, l})
		if mf := msysPath(f); mf != "" {
			ml := l
			if isWindowsPath(l) {
				ml = msysPath(l) // UNC 等没有 Git Bash 写法的，返回空，跳过
			}
			if mk := strings.ToLower(mf); ml != "" && !strings.EqualFold(mf, ml) && !seen[mk] {
				seen[mk] = true
				list = append(list, pair{mf, ml})
			}
		}
	}
	if len(list) == 0 {
		return nil
	}
	sort.SliceStable(list, func(i, j int) bool { return len(list[i].foreign) > len(list[j].foreign) })
	t := &translator{}
	for _, p := range list {
		t.rules = append(t.rules, translateRule{
			// "根路径 + (分隔符 + 这个字符串剩下的部分)? + 结尾引号"：整个字符串值都匹配进来，跨系统时要改后半段
			re:      regexp.MustCompile(`(?i)"` + rootPattern(p.foreign) + `((?:\\\\|/)(?:[^"\\]|\\.)*)?"`),
			local:   p.local,
			crossOS: isWindowsPath(p.foreign) != isWindowsPath(p.local),
		})
	}
	return t
}

// isWindowsPath 带盘符（C:）、带反斜杠或 UNC 的算 Windows 写法，其他（/Users/...）算 Unix 写法。
func isWindowsPath(p string) bool {
	return strings.Contains(p, `\`) || len(p) >= 2 && p[1] == ':' && (p[0] >= 'a' && p[0] <= 'z' || p[0] >= 'A' && p[0] <= 'Z')
}

// msysPath Windows 路径的 Git Bash 写法：D:\code\x → /d/code/x；不带盘符的返回空。
func msysPath(p string) string {
	if len(p) < 2 || p[1] != ':' || !(p[0] >= 'a' && p[0] <= 'z' || p[0] >= 'A' && p[0] <= 'Z') {
		return ""
	}
	return "/" + strings.ToLower(p[:1]) + strings.ReplaceAll(p[2:], `\`, "/")
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
	for _, r := range t.rules {
		backslash := []byte(`"` + strings.ReplaceAll(r.local, `\`, `\\`))
		slash := []byte(`"` + strings.ReplaceAll(r.local, `\`, `/`))
		re, crossOS, localWin := r.re, r.crossOS, isWindowsPath(r.local)
		data = re.ReplaceAllFunc(data, func(m []byte) []byte {
			sub := re.FindSubmatchIndex(m)
			rest := []byte{}
			if sub[2] >= 0 {
				rest = m[sub[2]:sub[3]]
			}
			head := m[:len(m)-len(rest)-1] // 引号 + 根路径
			out := append([]byte{}, slash...)
			switch {
			case crossOS && localWin:
				out = append([]byte{}, backslash...)
				rest = bytes.ReplaceAll(rest, []byte("/"), []byte(`\\`))
			case crossOS:
				rest = bytes.ReplaceAll(rest, []byte(`\\`), []byte("/"))
			case bytes.Contains(head, []byte(`\\`)) || bytes.HasPrefix(rest, []byte(`\\`)):
				out = append([]byte{}, backslash...)
			}
			return append(append(out, rest...), '"')
		})
	}
	return data
}

func isJSONish(rel string) bool {
	l := strings.ToLower(rel)
	return strings.HasSuffix(l, ".jsonl") || strings.HasSuffix(l, ".json")
}
