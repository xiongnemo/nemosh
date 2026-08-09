//go:build !windows

package main

import "strings"

// Everywhere but Windows a filesystem distinguishes case, so completion does
// too: `Makefile` and `makefile` can both exist and mean different things.
func completionMatches(name, stem string) bool {
	return strings.HasPrefix(name, stem)
}
