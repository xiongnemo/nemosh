//go:build windows

package main

import "strings"

// indexKey folds case, because the filesystem does. Typing `WSL` must find
// `wsl.exe`, the same reason completion matches without regard to case here.
func indexKey(name string) string {
	return strings.ToLower(name)
}
