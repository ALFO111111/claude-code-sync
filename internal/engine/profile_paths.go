package engine

import (
	"path/filepath"
	"sort"
	"strings"
)

const mcpHomePlaceholder = "${CLAUDE_SYNC_HOME}"
const mcpProjectPrefix = "${CLAUDE_SYNC_PROJECT:"
const mcpProjectSuffix = "}"

type mcpRoot struct {
	root        string
	placeholder string
}

func (e *Engine) mcpToCloud(path string) (string, bool) {
	if strings.HasPrefix(path, mcpHomePlaceholder) || strings.HasPrefix(path, mcpProjectPrefix) {
		return path, true
	}
	roots := []mcpRoot{{root: e.cfg.Home, placeholder: mcpHomePlaceholder}}
	for pid, binding := range e.st.Bindings {
		roots = append(roots, mcpRoot{root: binding.Root, placeholder: mcpProjectPrefix + pid + mcpProjectSuffix})
	}
	// 项目通常在用户目录里面，先匹配更长的根路径，避免被 home 占位符吞掉。
	sort.SliceStable(roots, func(i, j int) bool { return len(roots[i].root) > len(roots[j].root) })
	for _, candidate := range roots {
		if rel, ok := pathWithin(candidate.root, path); ok {
			return joinPlaceholder(candidate.placeholder, rel), true
		}
	}
	return "", false
}

func (e *Engine) mcpToLocal(path string) (string, bool) {
	// 占位符后面的部分一律用 / 分隔：出现 \ 或 :（盘符）就是不合法的，macOS 上 filepath.IsLocal 拦不住它们
	rest := path
	if i := strings.Index(path, "}"); i >= 0 {
		rest = path[i+1:]
	}
	if strings.ContainsAny(rest, `\:`) {
		return "", false
	}
	if path == mcpHomePlaceholder {
		return e.cfg.Home, true
	}
	if strings.HasPrefix(path, mcpHomePlaceholder+"/") {
		rel := strings.TrimPrefix(path, mcpHomePlaceholder+"/")
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return "", false
		}
		return filepath.Join(e.cfg.Home, filepath.FromSlash(rel)), true
	}
	if strings.HasPrefix(path, mcpProjectPrefix) {
		end := strings.Index(path, mcpProjectSuffix)
		if end < 0 {
			return "", false
		}
		pid := path[len(mcpProjectPrefix):end]
		binding := e.st.Bindings[pid]
		if binding == nil {
			return "", false
		}
		rest := path[end+1:]
		if rest == "" {
			return binding.Root, true
		}
		if !strings.HasPrefix(rest, "/") {
			return "", false
		}
		rel := strings.TrimPrefix(rest, "/")
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return "", false
		}
		return filepath.Join(binding.Root, filepath.FromSlash(rel)), true
	}
	return "", false
}

func pathWithin(root, path string) (string, bool) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	if strings.EqualFold(root, path) {
		return "", true
	}
	prefix := root + string(filepath.Separator)
	if len(path) <= len(prefix) || !strings.EqualFold(path[:len(prefix)], prefix) {
		return "", false
	}
	return path[len(prefix):], true
}

func joinPlaceholder(placeholder, rel string) string {
	if rel == "" {
		return placeholder
	}
	return placeholder + "/" + filepath.ToSlash(rel)
}
