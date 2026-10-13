package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ALFO111111/claude-code-sync/internal/fsutil"
	"github.com/ALFO111111/claude-code-sync/internal/i18n"
	"github.com/ALFO111111/claude-code-sync/internal/project"
)

// identify 根据工作目录算出项目身份证。
func (e *Engine) identify(cwd, dirName string) (pid, display string) {
	if fsutil.SamePath(cwd, e.cfg.Home) {
		return project.HomePID, i18n.T("用户目录（未打开项目）")
	}
	if pid, _, ok := project.GitIdentity(cwd); ok {
		return pid, filepath.Base(cwd)
	}
	return e.localPID(dirName), filepath.Base(cwd)
}

func (e *Engine) localPID(dirName string) string {
	return "local:" + e.cfg.DeviceID + ":" + strings.ToLower(dirName)
}

// localDirFor 返回某个工作目录在 ~/.claude/projects 下的文件夹名；已存在（大小写不同）则沿用。
func (e *Engine) localDirFor(root string) string {
	want := project.EncodeDir(root)
	entries, _ := os.ReadDir(e.cfg.ProjectsDir())
	for _, ent := range entries {
		if ent.IsDir() && strings.EqualFold(ent.Name(), want) {
			return ent.Name()
		}
	}
	return want
}
