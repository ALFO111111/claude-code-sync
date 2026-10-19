//go:build !windows

package platform

import "errors"

var errUnsupported = errors.New("only Windows is supported for now")

func autostart() (string, bool)                       { return "", false }
func setAutostart(string) error                       { return errUnsupported }
func removeAutostart() error                          { return nil }
func openFolder(string) error                         { return errUnsupported }
func openTerminal(dir string, command []string) error { return errUnsupported }
func darkMode() bool                                  { return false }
