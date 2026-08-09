//go:build windows

package main

import "strings"

// Windows decides what is runnable by suffix, so a file without one is not a
// program and is not indexed. A directory full of documents on PATH -- which
// happens -- would otherwise be offered as commands.
const runsWithoutASuffix = false

// indexKey folds case, because the filesystem does. Typing `WSL` must find
// `wsl.exe`, the same reason completion matches without regard to case here.
func indexKey(name string) string {
	return strings.ToLower(name)
}
