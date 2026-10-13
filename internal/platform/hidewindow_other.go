//go:build !windows

package platform

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
