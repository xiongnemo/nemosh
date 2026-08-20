package main

import (
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// Laying the choices out in columns rather than running them together is what
// makes a long list readable at all.
//
// It matters more here than in busybox, whose showfiles this follows
// (libbb/lineedit.c:1279): command completion reaches PATH, so `w` has a hundred
// and eighteen answers on an ordinary Windows machine where busybox would have
// had a handful. Joined by two blanks and left to wrap, that is a wall; in
// columns it is ten rows.
//
// Column-major, as busybox does it, so reading down a column is alphabetical.

// listedRowLimit is how much of the screen a listing may take.
//
// A budget in rows rather than a count of names, because rows are what the
// reader pays. Fifteen leaves the previous command and its output in view on an
// ordinary terminal, which is the thing a listing must not scroll away.
const listedRowLimit = 15

// candidateListing renders the choices, or reports why it did not. Kept for the
// callers that cannot ask a question -- see layoutCandidates for the rest.
func candidateListing(matches []string, width int) string {
	listing, rows := layoutCandidates(matches, width)
	if rows > listedRowLimit {
		return fmt.Sprintf("%d matches; type more to narrow\n", len(matches))
	}
	return listing
}

// layoutCandidates renders the choices and reports how many rows that took, so
// a caller can decide whether to print them or ask first.
func layoutCandidates(matches []string, width int) (string, int) {
	lines, rows := textgrid.Grid(matches, width)
	if rows == 0 {
		return "", 0
	}
	return strings.Join(lines, "\n") + "\n", rows
}
