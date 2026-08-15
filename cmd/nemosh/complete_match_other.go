//go:build !windows

package main

import "strings"

// Everywhere but Windows a filesystem distinguishes case, so completion does
// too: `Makefile` and `makefile` can both exist and mean different things.
func completionMatches(name, stem string) bool {
	return strings.HasPrefix(name, stem)
}

// foldForCompletion is the identity here, and is still worth having: it is the
// one place the case rule is written, so the shared prefix and the suggestion
// cannot each invent their own -- which is exactly what had happened on the
// Windows side of this split.
func foldForCompletion(value string) string {
	return value
}
