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
	return strings.HasPrefix(foldForCompletion(name), foldForCompletion(stem))
}

// foldForCompletion is that same rule as one function, so everything deciding
// whether two names are "the same" here decides it the same way.
//
// It exists because they did not. Matching folded case and the shared prefix did
// not, so `wh` -- eight matching commands on a real PATH -- inserted nothing,
// because one of the eight was spelled `WhoUses`. The grey suggestion, folding a
// third way, was offering `who` at the same moment.
func foldForCompletion(value string) string {
	return strings.ToLower(value)
}
