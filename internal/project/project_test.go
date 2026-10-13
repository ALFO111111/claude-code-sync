package project

import "testing"

// 同一个仓库的不同写法，归一化后必须相同（否则两台设备对不上，自动绑定失效）。
func TestNormalizeRemoteEquivalentForms(t *testing.T) {
	want := "github.com/owner/repo"
	forms := []string{
		"git@github.com:owner/repo.git",
		"https://github.com/owner/repo",
		"https://github.com/owner/repo.git",
		"ssh://git@github.com/owner/repo.git",
		"ssh://git@github.com:22/owner/repo.git", // 显式端口要剥掉
		"https://github.com:443/owner/repo",
		"Github.com/Owner/Repo/",
	}
	for _, f := range forms {
		if got := NormalizeRemote(f); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, 想要 %q", f, got, want)
		}
	}
}

func TestNormalizeRemoteNonDefaultPortStripped(t *testing.T) {
	// 两台设备一台带端口一台不带，也应认成同一个仓库
	a := NormalizeRemote("https://gitlab.example.com:8443/group/repo.git")
	b := NormalizeRemote("https://gitlab.example.com/group/repo.git")
	if a != b {
		t.Errorf("带端口 %q 和不带端口 %q 归一化结果应相同", a, b)
	}
}
