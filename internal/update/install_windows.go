//go:build windows

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func downloadDir(exe string) string { return filepath.Dir(exe) }

func apply(exe, downloaded string) (string, error) { return exe, replaceFile(exe, downloaded) }

func cleanupOld(exe string) { os.Remove(exe + ".old") }

// detach 新进程自成一组：旧进程退出时不会连带它。
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
