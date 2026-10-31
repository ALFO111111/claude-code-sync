//go:build !windows && !darwin

package update

import (
	"os"
	"os/exec"
	"path/filepath"
)

func downloadDir(exe string) string { return filepath.Dir(exe) }

func apply(exe, downloaded string) (string, error) {
	os.Remove(downloaded)
	return "", ErrNoAsset
}

func cleanupOld(string) {}

func detach(*exec.Cmd) {}
