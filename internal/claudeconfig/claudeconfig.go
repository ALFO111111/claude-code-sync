// Package claudeconfig 把 Claude 的完整本机配置转换成可同步的安全投影，再应用回来。
// 云端永远只包含明确允许的字段；供应商、凭据、hooks、权限等本机字段不离开本机。
package claudeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/i18n"
)

const Version = 1

var SettingsFields = []string{"autoMemoryEnabled", "includeCoAuthoredBy", "language"}

var mcpFields = map[string]bool{"type": true, "command": true, "args": true, "url": true}

// PathMapper 把本机绝对路径换成占位符，或把占位符换成本机路径。false 表示无法安全映射。
type PathMapper func(string) (string, bool)

func decode(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New(i18n.T("配置顶层必须是 JSON 对象"))
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New(i18n.T("配置 JSON 后面有多余内容"))
	}
	return m, nil
}

func encode(m map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	return append(data, '\n'), err
}

// SettingsProjection 从完整 settings.json 提取白名单。
func SettingsProjection(full []byte) ([]byte, error) {
	m, err := decode(full)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	for _, key := range SettingsFields {
		if value, ok := m[key]; ok {
			values[key] = value
		}
	}
	return encode(map[string]any{"version": Version, "settings": values})
}

// ApplySettings 只替换投影中的白名单字段，完整本机配置的其他内容原样保留。
func ApplySettings(full, projection []byte) ([]byte, error) {
	m, err := decode(full)
	if err != nil {
		return nil, err
	}
	p, err := decode(projection)
	if err != nil {
		return nil, err
	}
	values, ok := p["settings"].(map[string]any)
	if !ok {
		return nil, errors.New(i18n.T("settings 同步内容缺少 settings 对象"))
	}
	for _, key := range SettingsFields {
		if value, ok := values[key]; ok {
			m[key] = value
		} else {
			delete(m, key)
		}
	}
	return encode(m)
}

// MCPProjection 提取用户级 mcpServers。秘密字段和无法映射的路径不会进入返回值。
func MCPProjection(full []byte, mapPath PathMapper) ([]byte, []string, error) {
	m, err := decode(full)
	if err != nil {
		return nil, nil, err
	}
	servers, _ := m["mcpServers"].(map[string]any)
	out := map[string]any{}
	var skipped []string
	for name, raw := range servers {
		server, ok := raw.(map[string]any)
		if !ok {
			skipped = append(skipped, name)
			continue
		}
		clean, ok := portableServer(server, mapPath)
		if !ok {
			skipped = append(skipped, name)
			continue
		}
		out[name] = clean
	}
	sort.Strings(skipped)
	data, err := encode(map[string]any{"version": Version, "mcpServers": out})
	return data, skipped, err
}

func portableServer(server map[string]any, mapPath PathMapper) (map[string]any, bool) {
	out := map[string]any{}
	for key, value := range server {
		if !mcpFields[key] {
			continue // env、headers 以及未来新增字段一律 default-deny
		}
		mapped, ok := portableValue(value, mapPath)
		if !ok {
			return nil, false
		}
		out[key] = mapped
	}
	if raw, ok := out["url"].(string); ok && secretURL(raw) {
		return nil, false
	}
	return out, true
}

func portableValue(value any, mapPath PathMapper) (any, bool) {
	switch v := value.(type) {
	case string:
		if looksSecret(v) {
			return nil, false
		}
		if isAbsAnyOS(v) || strings.Contains(v, "${CLAUDE_SYNC_") {
			if mapPath == nil {
				return nil, false
			}
			mapped, ok := mapPath(v)
			if !ok {
				return nil, false
			}
			return mapped, true
		}
		return v, true
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			mapped, ok := portableValue(item, mapPath)
			if !ok {
				return nil, false
			}
			out[i] = mapped
		}
		return out, true
	case map[string]any:
		// 目前允许的 MCP 字段里没有需要对象值；拒绝比猜测字段所有权安全。
		return nil, false
	default:
		return value, true
	}
}

func looksSecret(s string) bool {
	lower := strings.ToLower(s)
	for _, word := range []string{"--api-key", "--apikey", "--access-token", "--auth-token", "--password", "--secret"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return strings.HasPrefix(lower, "sk-") && len(s) >= 16
}

func secretURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	if u.User != nil {
		return true
	}
	for key := range u.Query() {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") {
			return true
		}
	}
	return false
}

// ApplyMCP 应用可移植字段；同名服务器已有的 env、headers 和其他本机字段继续保留。
func ApplyMCP(full, projection []byte, mapPath PathMapper) ([]byte, []string, error) {
	m, err := decode(full)
	if err != nil {
		return nil, nil, err
	}
	p, err := decode(projection)
	if err != nil {
		return nil, nil, err
	}
	incoming, ok := p["mcpServers"].(map[string]any)
	if !ok {
		return nil, nil, errors.New(i18n.T("MCP 同步内容缺少 mcpServers 对象"))
	}
	local, _ := m["mcpServers"].(map[string]any)
	if local == nil {
		local = map[string]any{}
	}
	out := map[string]any{}
	var skipped []string
	for name, raw := range incoming {
		server, ok := raw.(map[string]any)
		if !ok {
			skipped = append(skipped, name)
			if old, exists := local[name]; exists {
				out[name] = old
			}
			continue
		}
		clean, ok := portableServer(server, mapPath)
		if !ok {
			skipped = append(skipped, name)
			if old, exists := local[name]; exists {
				out[name] = old
			}
			continue
		}
		merged := map[string]any{}
		if old, _ := local[name].(map[string]any); old != nil {
			for key, value := range old {
				if !mcpFields[key] {
					merged[key] = value
				}
			}
		}
		for key, value := range clean {
			merged[key] = value
		}
		out[name] = merged
	}
	sort.Strings(skipped)
	if len(out) == 0 {
		delete(m, "mcpServers")
	} else {
		m["mcpServers"] = out
	}
	data, err := encode(m)
	return data, skipped, err
}

func ValidateSettingsProjection(data []byte) error {
	_, err := ApplySettings([]byte("{}"), data)
	return err
}

func ValidateMCPProjection(data []byte) error {
	_, _, err := ApplyMCP([]byte("{}"), data, nil)
	return err
}

func DescribeSkipped(kind string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	return fmt.Sprintf(i18n.T("%s 中有 %d 项含秘密或无法映射的本机路径，已保留在本机：%s"), kind, len(names), strings.Join(names, ", "))
}

// isAbsAnyOS 本机的绝对路径，或者 Windows 写法的绝对路径（C:\x、\\server\x）：
// macOS 上 filepath.IsAbs 不认 C:\，从 Windows 拷来的配置里的路径会被当成普通文字原样上传。
// 反过来不把 /x 当路径：Windows 上 MCP 参数里常见 cmd /c 这种开关。
func isAbsAnyOS(v string) bool {
	if filepath.IsAbs(v) || strings.HasPrefix(v, `\\`) {
		return true
	}
	return len(v) >= 3 && v[1] == ':' && (v[2] == '\\' || v[2] == '/') && (v[0]|0x20 >= 'a' && v[0]|0x20 <= 'z')
}
