package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

// mergeText3 用共同版本合并两份文本。返回 conflict=true 时不返回可用的合并结果。
// 实现采用行级 LCS/diff3：两边改不同区域时自动合并，同一区域改成不同内容时交给用户。
func mergeText3(base, local, remote []byte) (merged []byte, conflict bool) {
	if bytes.Equal(local, remote) {
		return local, false
	}
	if bytes.Equal(local, base) {
		return remote, false
	}
	if bytes.Equal(remote, base) {
		return local, false
	}
	if !validText(base) || !validText(local) || !validText(remote) {
		return nil, true
	}
	b, l, r := splitMergeLines(base), splitMergeLines(local), splitMergeLines(remote)
	le, ok := lineEdits(b, l)
	if !ok {
		return nil, true
	}
	re, ok := lineEdits(b, r)
	if !ok {
		return nil, true
	}
	lines, ok := mergeLineEdits(b, le, re)
	if !ok {
		return nil, true
	}
	var out bytes.Buffer
	for _, line := range lines {
		out.WriteString(line)
	}
	return out.Bytes(), false
}

func validText(b []byte) bool {
	return utf8.Valid(b) && !bytes.ContainsRune(b, '\x00')
}

func splitMergeLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := stringsSplitAfter(string(b))
	return lines
}

// stringsSplitAfter 保留每行换行符，合并后不会悄悄改变最后一行是否有换行。
func stringsSplitAfter(s string) []string {
	var out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

type lineEdit struct {
	start, end int
	repl       []string
}

// 超过这个规模的文件不做 O(n*m) 的 LCS，宁可让用户处理，也不让同步占用大量内存。
const maxLCSCells = 4_000_000

func lineEdits(base, side []string) ([]lineEdit, bool) {
	n, m := len(base), len(side)
	if n != 0 && m > maxLCSCells/n {
		return nil, false
	}
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if base[i] == side[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var edits []lineEdit
	for i, j := 0, 0; i < n || j < m; {
		if i < n && j < m && base[i] == side[j] && dp[i][j] == dp[i+1][j+1]+1 {
			i++
			j++
			continue
		}
		si, sj := i, j
		for i < n || j < m {
			if i < n && j < m && base[i] == side[j] && dp[i][j] == dp[i+1][j+1]+1 {
				break
			}
			if j < m && (i == n || dp[i+1][j] <= dp[i][j+1]) {
				j++
			} else if i < n {
				i++
			}
		}
		edits = append(edits, lineEdit{start: si, end: i, repl: append([]string(nil), side[sj:j]...)})
	}
	return edits, true
}

func editsOverlap(a, b lineEdit) bool {
	aIns, bIns := a.start == a.end, b.start == b.end
	switch {
	case aIns && bIns:
		return a.start == b.start
	case aIns:
		return a.start >= b.start && a.start < b.end
	case bIns:
		return b.start >= a.start && b.start < a.end
	default:
		return a.start < b.end && b.start < a.end
	}
}

func mergeLineEdits(base []string, local, remote []lineEdit) ([]string, bool) {
	var out []string
	pos, li, ri := 0, 0, 0
	appendEdit := func(e lineEdit) {
		out = append(out, base[pos:e.start]...)
		out = append(out, e.repl...)
		pos = e.end
	}
	for li < len(local) || ri < len(remote) {
		if li >= len(local) {
			appendEdit(remote[ri])
			ri++
			continue
		}
		if ri >= len(remote) {
			appendEdit(local[li])
			li++
			continue
		}
		l, r := local[li], remote[ri]
		if !editsOverlap(l, r) {
			if l.start < r.start || (l.start == r.start && l.end <= r.end) {
				appendEdit(l)
				li++
			} else {
				appendEdit(r)
				ri++
			}
			continue
		}

		start, end := l.start, l.end
		if r.start < start {
			start = r.start
		}
		if r.end > end {
			end = r.end
		}
		ls := []lineEdit{l}
		rs := []lineEdit{r}
		li++
		ri++
		changed := true
		for changed {
			changed = false
			for li < len(local) {
				e := local[li]
				overlap := false
				for _, other := range rs {
					if editsOverlap(e, other) {
						overlap = true
						break
					}
				}
				if !overlap {
					break
				}
				ls = append(ls, e)
				if e.start < start {
					start = e.start
				}
				if e.end > end {
					end = e.end
				}
				li++
				changed = true
			}
			for ri < len(remote) {
				e := remote[ri]
				overlap := false
				for _, other := range ls {
					if editsOverlap(e, other) {
						overlap = true
						break
					}
				}
				if !overlap {
					break
				}
				rs = append(rs, e)
				if e.start < start {
					start = e.start
				}
				if e.end > end {
					end = e.end
				}
				ri++
				changed = true
			}
		}
		lv := applyLineEdits(base, ls, start, end)
		rv := applyLineEdits(base, rs, start, end)
		if !reflect.DeepEqual(lv, rv) {
			return nil, false
		}
		out = append(out, base[pos:start]...)
		out = append(out, lv...)
		pos = end
	}
	out = append(out, base[pos:]...)
	return out, true
}

func applyLineEdits(base []string, edits []lineEdit, start, end int) []string {
	var out []string
	pos := start
	for _, e := range edits {
		out = append(out, base[pos:e.start]...)
		out = append(out, e.repl...)
		pos = e.end
	}
	out = append(out, base[pos:end]...)
	return out
}

// mergeJSON3 对 JSON 做结构化三方合并。keyedArrays=true 时，元素都带字符串 id 的数组按 id 合并。
func mergeJSON3(base, local, remote []byte, keyedArrays bool) ([]byte, bool, error) {
	b, err := decodeJSONValue(base)
	if err != nil {
		return nil, false, err
	}
	l, err := decodeJSONValue(local)
	if err != nil {
		return nil, false, err
	}
	r, err := decodeJSONValue(remote)
	if err != nil {
		return nil, false, err
	}
	v, conflict := mergeJSONValue(b, l, r, keyedArrays)
	if conflict {
		return nil, true, nil
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), false, nil
}

func decodeJSONValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errInvalidJSON
		}
		return nil, err
	}
	return v, nil
}

var errInvalidJSON = errors.New("invalid JSON")

type missingJSON struct{}

var jsonMissing = missingJSON{}

func mergeJSONValue(base, local, remote any, keyedArrays bool) (any, bool) {
	if reflect.DeepEqual(local, remote) {
		return local, false
	}
	if reflect.DeepEqual(local, base) {
		return remote, false
	}
	if reflect.DeepEqual(remote, base) {
		return local, false
	}
	bm, bok := jsonObject(base)
	lm, lok := jsonObject(local)
	rm, rok := jsonObject(remote)
	if (bok || isJSONMissing(base)) && (lok || isJSONMissing(local)) && (rok || isJSONMissing(remote)) {
		if !bok {
			bm = map[string]any{}
		}
		if !lok {
			lm = map[string]any{}
		}
		if !rok {
			rm = map[string]any{}
		}
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range lm {
			keys[k] = true
		}
		for k := range rm {
			keys[k] = true
		}
		ordered := make([]string, 0, len(keys))
		for k := range keys {
			ordered = append(ordered, k)
		}
		sort.Strings(ordered)
		out := map[string]any{}
		for _, k := range ordered {
			v, conflict := mergeJSONValue(mapValue(bm, k), mapValue(lm, k), mapValue(rm, k), keyedArrays)
			if conflict {
				return nil, true
			}
			if !isJSONMissing(v) {
				out[k] = v
			}
		}
		return out, false
	}
	if keyedArrays {
		ba, bok := jsonArray(base)
		la, lok := jsonArray(local)
		ra, rok := jsonArray(remote)
		if (bok || isJSONMissing(base)) && (lok || isJSONMissing(local)) && (rok || isJSONMissing(remote)) {
			return mergeKeyedArrays(ba, la, ra)
		}
	}
	return nil, true
}

func jsonObject(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func jsonArray(v any) ([]any, bool) {
	a, ok := v.([]any)
	return a, ok
}

func isJSONMissing(v any) bool {
	_, ok := v.(missingJSON)
	return ok
}

func mapValue(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	return jsonMissing
}

func mergeKeyedArrays(base, local, remote []any) (any, bool) {
	bm, ok := keyedArrayMap(base)
	if !ok {
		return nil, true
	}
	lm, ok := keyedArrayMap(local)
	if !ok {
		return nil, true
	}
	rm, ok := keyedArrayMap(remote)
	if !ok {
		return nil, true
	}
	merged := map[string]any{}
	for id := range bm {
		v, conflict := mergeJSONValue(bm[id], mapValue(lm, id), mapValue(rm, id), true)
		if conflict {
			return nil, true
		}
		if !isJSONMissing(v) {
			merged[id] = v
		}
	}
	for id := range lm {
		if _, done := merged[id]; done {
			continue
		}
		v, conflict := mergeJSONValue(mapValue(bm, id), lm[id], mapValue(rm, id), true)
		if conflict {
			return nil, true
		}
		if !isJSONMissing(v) {
			merged[id] = v
		}
	}
	for id := range rm {
		if _, done := merged[id]; done {
			continue
		}
		v, conflict := mergeJSONValue(mapValue(bm, id), mapValue(lm, id), rm[id], true)
		if conflict {
			return nil, true
		}
		if !isJSONMissing(v) {
			merged[id] = v
		}
	}
	out := []any{}
	seen := map[string]bool{}
	for _, arr := range [][]any{local, remote} {
		for _, item := range arr {
			id := item.(map[string]any)["id"].(string)
			if v, exists := merged[id]; exists && !seen[id] {
				out = append(out, v)
				seen[id] = true
			}
		}
	}
	return out, false
}

func keyedArrayMap(arr []any) (map[string]any, bool) {
	out := map[string]any{}
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		id, ok := m["id"].(string)
		if !ok || id == "" {
			return nil, false
		}
		if _, exists := out[id]; exists {
			return nil, false
		}
		out[id] = item
	}
	return out, true
}
