//go:build windows

package main

import (
	"sort"
	"strings"
)

// Candidates are ordered without regard to case, because they are matched
// without regard to it.
//
// Byte order would be a poor answer where the two disagree: on Windows the PATH
// is full of system programs spelled in capitals, so `w` listed `WFS WMIADAP
// WMIC WMPDMC` before it ever reached `wait` and `wc`. Sorting the way the
// matching works puts them where a reader would look for them.
func sortCandidates(matches []string) {
	sort.Slice(matches, func(i, j int) bool {
		left, right := strings.ToLower(matches[i]), strings.ToLower(matches[j])
		if left != right {
			return left < right
		}
		// Two spellings of one name still need a stable order between them.
		return matches[i] < matches[j]
	})
}
