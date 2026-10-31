//go:build !windows && !darwin

package update

func systemProxy() string { return "" }
