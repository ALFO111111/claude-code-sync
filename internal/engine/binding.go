package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/config"
	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/project"
	"github.com/ALFO111111/claude-code-sync/internal/state"
)

// 项目发现与绑定：本机项目目录 ↔ 项目身份证（pid）的对应关系。

// discover 扫描本机 ~/.claude/projects，给新出现的目录认身份。
func (e *Engine) discover() {
	entries, err := os.ReadDir(e.cfg.ProjectsDir())
	if err != nil {
		return
	}
	byDir := map[string]bool{}
	for _, b := range e.st.Bindings {
		byDir[strings.ToLower(b.Dir)] = true
	}
	for _, ent := range entries {
		name := ent.Name()
		if !ent.IsDir() || byDir[strings.ToLower(name)] || e.cfg.Ignored(name) {
			continue
		}
		cwd := project.ReadCwd(filepath.Join(e.cfg.ProjectsDir(), name))
		if cwd == "" {
			continue
		}
		pid, display := e.identify(cwd, name)
		if e.cfg.Ignored(pid) || e.st.Ignored[pid] {
			continue
		}
		if b, ok := e.st.Bindings[pid]; ok && !strings.EqualFold(b.Dir, name) {
			e.log.Printf("%s 和 %s 是同一个仓库，后者作为独立项目同步", b.Root, cwd)
			pid = e.localPID(name)
		}
		e.st.Bindings[pid] = &state.Binding{PID: pid, Root: cwd, Dir: name, Display: display}
		byDir[strings.ToLower(name)] = true
		e.log.Printf("发现项目 %s（%s）", cwd, pid)
	}
}

// autoBind 自动对上其他设备的项目：用户目录直接对上；git 项目在本机找同一个仓库。
func (e *Engine) autoBind() {
	var repos map[string][]string
	for pid, srcs := range e.remotePIDs() {
		if e.st.Bindings[pid] != nil || e.st.Ignored[pid] || e.cfg.Ignored(pid) {
			continue
		}
		var root string
		switch project.Kind(pid) {
		case "home":
			root = e.cfg.Home
		case "git":
			if repos == nil {
				repos = e.scanRepos()
			}
			base, sub, _ := strings.Cut(strings.TrimPrefix(pid, "git:"), "#")
			tops := repos[base]
			if len(tops) == 0 {
				continue
			}
			if len(tops) > 1 {
				// 本机有多个同一仓库的克隆，不知道会话属于哪个，交给用户 claim 时选择
				e.log.Printf("项目 %s 在本机有 %d 个克隆，请用 %s claim %s <路径> 指定一个", pid, len(tops), config.AppName, pid)
				continue
			}
			root = tops[0]
			if sub != "" {
				root = filepath.Join(root, filepath.FromSlash(sub))
			}
			if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
				continue
			}
		default:
			continue
		}
		if e.bind(pid, root, srcs[0].Info.Display, false) != nil {
			e.log.Printf("自动对上项目 %s → %s", pid, root)
		}
	}
}

// scanRepos 在可能的位置找 git 仓库，返回 仓库地址 → 本机仓库根目录。
// 搜索位置：配置的 scanRoots、本机已知项目的上级目录、其他设备上同名仓库所在的上级目录、用户目录。
func (e *Engine) scanRepos() map[string][]string {
	cands := map[string]string{}
	add := func(p string) {
		if p != "" {
			cands[strings.ToLower(fsutil.CleanPath(p))] = p
		}
	}
	for _, p := range e.cfg.ScanRoots {
		add(p)
	}
	for _, b := range e.st.Bindings {
		if b.PID != project.HomePID {
			add(filepath.Dir(b.Root))
		}
	}
	for pid, srcs := range e.remotePIDs() {
		if project.Kind(pid) == "git" {
			for _, s := range srcs {
				add(filepath.Dir(s.Info.Root))
			}
		}
	}
	add(e.cfg.Home)

	out := map[string][]string{}
	seen := map[string]bool{}
	check := func(dir string) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			return
		}
		pid, top, ok := project.GitIdentity(dir)
		if !ok || seen[strings.ToLower(fsutil.CleanPath(top))] {
			return
		}
		seen[strings.ToLower(fsutil.CleanPath(top))] = true
		base, _, _ := strings.Cut(strings.TrimPrefix(pid, "git:"), "#")
		out[base] = append(out[base], top)
	}
	for _, root := range cands {
		check(root)
		entries, _ := os.ReadDir(root)
		for _, ent := range entries {
			if ent.IsDir() && !strings.HasPrefix(ent.Name(), ".") {
				check(filepath.Join(root, ent.Name()))
			}
		}
	}
	return out
}

// bind 把项目身份证绑定到本机目录。takeover=true 时，若该目录已属于别的项目，则改归新项目（认领时用）。
func (e *Engine) bind(pid, root, display string, takeover bool) *state.Binding {
	dir := e.localDirFor(root)
	for opid, b := range e.st.Bindings {
		if opid != pid && strings.EqualFold(b.Dir, dir) {
			if !takeover {
				e.log.Printf("跳过 %s：本机目录 %s 已属于 %s", pid, root, opid)
				return nil
			}
			delete(e.st.Bindings, opid)
			e.dropFileStates(opid)
		}
	}
	b := &state.Binding{PID: pid, Root: root, Dir: dir, Display: display}
	e.st.Bindings[pid] = b
	return b
}

// ignore 忽略一个项目。已绑定的同时解绑：本机不再同步它，但本机会话文件、云端文件都原样保留。
// 必须同时记进"忽略"：只删绑定的话，下一轮 discover 扫到本机会话目录又会自动绑回来。
// 文件状态一并清掉：以后取消忽略、哪怕绑到别的目录，也按"第一次见面"重新比对，
// 不会因为旧状态把云端文件误判成"本地删了"。
func (e *Engine) ignore(pid string) {
	e.st.Ignored[pid] = true
	if e.st.Bindings[pid] != nil {
		delete(e.st.Bindings, pid)
		e.dropFileStates(pid)
		e.log.Printf("已停止同步项目 %s（本机和云端的文件都没动）", pid)
		return
	}
	e.log.Printf("已忽略项目 %s", pid)
}

func (e *Engine) dropFileStates(pid string) {
	prefix := "files/" + project.Hash(pid) + "/"
	for k := range e.st.Files {
		if strings.HasPrefix(k, prefix) {
			delete(e.st.Files, k)
		}
	}
}

// bindCwd 找到（或建立）某个工作目录的绑定，只在本机已有会话或云端有这个项目时才建立。
func (e *Engine) BindCwd(cwd string) *state.Binding {
	for _, b := range e.st.Bindings {
		if fsutil.SamePath(b.Root, cwd) {
			return b
		}
	}
	dir := e.localDirFor(cwd)
	pid, display := e.identify(cwd, dir)
	if e.cfg.Ignored(pid, dir) || e.st.Ignored[pid] {
		return nil
	}
	if e.st.Bindings[pid] != nil {
		return nil // 同一个仓库在本机另一个位置已经绑定了
	}
	_, statErr := os.Stat(filepath.Join(e.cfg.ProjectsDir(), dir))
	if statErr != nil && e.remotePIDs()[pid] == nil {
		return nil
	}
	return e.bind(pid, cwd, display, false)
}
