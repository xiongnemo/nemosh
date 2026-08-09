//go:build windows

package main

import "strings"

// Completion ignores case on Windows because the filesystem does: typing `prog`
// and being offered nothing while `Program Files` sits right there is the shell
// contradicting the thing it is listing. busybox-w32 makes the same split,
// comparing with strncasecmp where the portable build uses strncmp
// (libbb/lineedit.c:1039).
//
// ToLower rather than slicing to len(stem) and folding: a byte slice can cut a
// multi-byte rune in half, and a name typed in Chinese would then match nothing
// or match wrongly.
func completionMatches(name, stem string) bool {
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(stem))
}
