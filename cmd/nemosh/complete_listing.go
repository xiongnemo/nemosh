package main

import (
	"fmt"
	"strings"
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
	if len(matches) == 0 {
		return "", 0
	}
	columnWidth := 0
	for _, match := range matches {
		if columns := textColumns(match); columns > columnWidth {
			columnWidth = columns
		}
	}
	columnWidth += 2
	columns := max(width/columnWidth, 1)
	rows := (len(matches) + columns - 1) / columns

	var out strings.Builder
	for row := range rows {
		for column := range columns {
			index := column*rows + row
			if index >= len(matches) {
				break
			}
			out.WriteString(matches[index])
			// No padding after the last column, so a row that fills the terminal
			// does not wrap into an empty one.
			if index+rows < len(matches) {
				out.WriteString(strings.Repeat(" ", columnWidth-textColumns(matches[index])))
			}
		}
		out.WriteString("\n")
	}
	return out.String(), rows
}

// textColumns is how many cells a candidate occupies, which is not its length
// once a name holds a wide character.
func textColumns(text string) int {
	total := 0
	for _, r := range text {
		total += runeColumns(r)
	}
	return total
}
