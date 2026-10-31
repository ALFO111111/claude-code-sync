package engine

import (
	"path/filepath"
	"testing"

	"github.com/ALFO111111/claude-code-sync/internal/state"
)

func bindMCPTestProject(e *Engine, pid, root string) {
	e.st.Bindings[pid] = &state.Binding{PID: pid, Root: root}
}

func TestMCPProfilePathHomeRoundTrip(t *testing.T) {
	e := newTestApp(t, "devA")
	for _, path := range []string{
		e.cfg.Home,
		filepath.Join(e.cfg.Home, "bin", "tool.exe"),
	} {
		cloud, ok := e.mcpToCloud(path)
		if !ok {
			t.Fatalf("Home 路径应能转成云端占位符：%q", path)
		}
		local, ok := e.mcpToLocal(cloud)
		if !ok || local != path {
			t.Errorf("Home 路径往返不一致：原路径 %q，云端 %q，还原本机 %q，ok=%v", path, cloud, local, ok)
		}
	}
}

func TestMCPProfilePathProjectAcrossRoots(t *testing.T) {
	a := newTestApp(t, "devA")
	rootA := filepath.Join(t.TempDir(), "device-a", "repo")
	bindMCPTestProject(a, "git:example.com/owner/repo", rootA)

	cloud, ok := a.mcpToCloud(filepath.Join(rootA, "bin", "tool.exe"))
	wantCloud := "${CLAUDE_SYNC_PROJECT:git:example.com/owner/repo}/bin/tool.exe"
	if !ok || cloud != wantCloud {
		t.Fatalf("项目路径应转成项目占位符：得到 %q，ok=%v；期望 %q", cloud, ok, wantCloud)
	}

	b := newTestApp(t, "devB")
	rootB := filepath.Join(t.TempDir(), "different", "location", "repo")
	bindMCPTestProject(b, "git:example.com/owner/repo", rootB)
	local, ok := b.mcpToLocal(cloud)
	wantLocal := filepath.Join(rootB, "bin", "tool.exe")
	if !ok || local != wantLocal {
		t.Errorf("同一项目占位符应落到另一台设备的不同 Root：得到 %q，ok=%v；期望 %q", local, ok, wantLocal)
	}
}

func TestMCPProfilePathLongestRoot(t *testing.T) {
	t.Run("项目位于 Home 内", func(t *testing.T) {
		e := newTestApp(t, "devA")
		root := filepath.Join(e.cfg.Home, "work", "repo")
		bindMCPTestProject(e, "project", root)

		cloud, ok := e.mcpToCloud(filepath.Join(root, "bin", "tool.exe"))
		want := "${CLAUDE_SYNC_PROJECT:project}/bin/tool.exe"
		if !ok || cloud != want {
			t.Errorf("应优先使用比 Home 更长的项目根：得到 %q，ok=%v；期望 %q", cloud, ok, want)
		}
	})

	t.Run("嵌套项目", func(t *testing.T) {
		e := newTestApp(t, "devA")
		outer := filepath.Join(t.TempDir(), "outer")
		inner := filepath.Join(outer, "nested", "inner")
		bindMCPTestProject(e, "outer", outer)
		bindMCPTestProject(e, "inner", inner)

		cloud, ok := e.mcpToCloud(filepath.Join(inner, "bin", "tool.exe"))
		want := "${CLAUDE_SYNC_PROJECT:inner}/bin/tool.exe"
		if !ok || cloud != want {
			t.Errorf("嵌套项目应使用最长根：得到 %q，ok=%v；期望 %q", cloud, ok, want)
		}
	})
}

func TestMCPProfilePathRejectsMalformedOrUnsafeCloudPaths(t *testing.T) {
	e := newTestApp(t, "devA")
	bindMCPTestProject(e, "known", filepath.Join(t.TempDir(), "repo"))

	tests := []struct {
		name string
		path string
	}{
		{name: "未知 pid", path: "${CLAUDE_SYNC_PROJECT:unknown}/tool.exe"},
		{name: "缺右花括号", path: "${CLAUDE_SYNC_PROJECT:known/tool.exe"},
		{name: "Home 占位符后无斜杠", path: "${CLAUDE_SYNC_HOME}tool.exe"},
		{name: "项目占位符后无斜杠", path: "${CLAUDE_SYNC_PROJECT:known}tool.exe"},
		{name: "Home 直接上级逃逸", path: "${CLAUDE_SYNC_HOME}/../outside.exe"},
		{name: "项目直接上级逃逸", path: "${CLAUDE_SYNC_PROJECT:known}/../outside.exe"},
		{name: "Home 嵌套上级逃逸", path: "${CLAUDE_SYNC_HOME}/safe/../../outside.exe"},
		{name: "项目嵌套上级逃逸", path: "${CLAUDE_SYNC_PROJECT:known}/safe/../../outside.exe"},
		{name: "Home 后接驱动器绝对路径", path: "${CLAUDE_SYNC_HOME}/C:/outside.exe"},
		{name: "项目后接驱动器绝对路径", path: "${CLAUDE_SYNC_PROJECT:known}/D:/outside.exe"},
		{name: "Home 后接 UNC 绝对路径", path: "${CLAUDE_SYNC_HOME}//server/share/tool.exe"},
		{name: "项目后接 UNC 绝对路径", path: "${CLAUDE_SYNC_PROJECT:known}//server/share/tool.exe"},
		{name: "不带占位符的绝对路径", path: `C:\outside.exe`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if local, ok := e.mcpToLocal(tt.path); ok {
				t.Errorf("不应接受不安全或格式错误的云端路径 %q，得到本机路径 %q", tt.path, local)
			}
		})
	}
}

func TestMCPProfilePathSimilarPrefixSibling(t *testing.T) {
	e := newTestApp(t, "devA")
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	bindMCPTestProject(e, "project", root)

	tests := []struct {
		name string
		root string
		path string
	}{
		{name: "项目的相似前缀兄弟目录", root: root, path: filepath.Join(parent, "repo-other", "tool.exe")},
		{name: "Home 的相似前缀兄弟目录", root: e.cfg.Home, path: e.cfg.Home + "-other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if cloud, ok := e.mcpToCloud(tt.path); ok {
				t.Errorf("相似前缀兄弟目录不应匹配根 %q：路径 %q 被转成 %q", tt.root, tt.path, cloud)
			}
		})
	}
}
