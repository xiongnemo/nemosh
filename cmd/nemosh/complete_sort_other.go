//go:build !windows

package main

import "sort"

// Byte order, because matching is by byte here too: `Makefile` and `makefile`
// are two different files and neither should be sorted as though it were the
// other.
func sortCandidates(matches []string) {
	sort.Strings(matches)
}
