package applets

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reading a unified diff, as busybox's patch_main reads one.
//
// A `--- ` line names a file's old side and `+++ ` its new one, and each `@@ -a,b +c,d @@` after
// them begins a hunk whose lines are counted off against b and d -- so a removed line that reads
// `--- x` is part of its hunk, not a header, where it began a file of its own when a hunk ran
// until the first line that was no body line. What lies between hunks, `diff --git` and `index`
// lines among it, is passed over. A `\ No newline at end of file` marks the line before it.

// patchDevNull is the name diff gives a side that is not there: a file being created, or removed.
const patchDevNull = "/dev/null"

// patchSet is every hunk for one file, and the names its --- and +++ lines gave it.
type patchSet struct {
	oldName string
	newName string
	hunks   []patchHunk
}

// patchHunk is one @@ block: where each side starts and how long it is, and the body, each line
// with its marker.
type patchHunk struct {
	oldStart, oldLen int
	newStart, newLen int
	lines            []string
	// oldNoNewline and newNoNewline say that side's last line has no newline after it.
	oldNoNewline, newNoNewline bool
	// short says the hunk ended before its counts did.
	short bool
}

// parseUnifiedPatch reads a unified diff into per-file hunk sets.
func parseUnifiedPatch(reader io.Reader) ([]patchSet, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var sets []patchSet
	oldName, inHunk, oldLeft, newLeft := "", false, 0, 0
	// ended says the line before closed a hunk, so a `\` now marks that hunk's last line.
	ended := false
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if inHunk {
			hunk := lastPatchHunk(sets)
			if line == "" {
				// A context line a mailer stripped of its blank, which busybox reads as one.
				line = " "
			}
			if line[0] == ' ' || line[0] == '-' || line[0] == '+' {
				hunk.lines = append(hunk.lines, line)
				oldLeft, newLeft = oldLeft-patchCount(line[0] != '+'), newLeft-patchCount(line[0] != '-')
				inHunk = oldLeft > 0 || newLeft > 0
				ended = !inHunk
				continue
			}
			if line[0] == '\\' {
				hunk.markNoNewline()
				continue
			}
			hunk.short, inHunk = true, false
		}
		if ended && strings.HasPrefix(line, `\`) {
			lastPatchHunk(sets).markNoNewline()
			ended = false
			continue
		}
		ended = false
		switch {
		case strings.HasPrefix(line, "--- "):
			oldName = patchName(line[4:])
		case strings.HasPrefix(line, "+++ "):
			sets = append(sets, patchSet{oldName: oldName, newName: patchName(line[4:])})
			oldName = ""
		case strings.HasPrefix(line, "@@ -") && len(sets) > 0:
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			set := &sets[len(sets)-1]
			set.hunks = append(set.hunks, hunk)
			inHunk, oldLeft, newLeft = true, hunk.oldLen, hunk.newLen
		}
	}
	if inHunk {
		lastPatchHunk(sets).short = true
	}
	return sets, scanner.Err()
}

func patchCount(counts bool) int {
	if counts {
		return 1
	}
	return 0
}

// lastPatchHunk is the hunk read last, or nil before the first.
func lastPatchHunk(sets []patchSet) *patchHunk {
	if len(sets) == 0 || len(sets[len(sets)-1].hunks) == 0 {
		return nil
	}
	hunks := sets[len(sets)-1].hunks
	return &hunks[len(hunks)-1]
}

// markNoNewline is a `\ No newline at end of file`: the line before it ends its side without
// one, the old side's for a removed line, the new side's for an added one, and both for context.
func (h *patchHunk) markNoNewline() {
	if len(h.lines) == 0 {
		return
	}
	marker := h.lines[len(h.lines)-1][0]
	h.oldNoNewline = h.oldNoNewline || marker != '+'
	h.newNoNewline = h.newNoNewline || marker != '-'
}

// patchName is the name on a --- or +++ line, which ends at a tab: diff dates a file after one.
// A date in 1970 or before is how `diff -N` dates a side that is not there, and busybox reads
// that side as /dev/null.
func patchName(rest string) string {
	name, date, dated := strings.Cut(rest, "\t")
	if dated {
		date = strings.TrimSpace(date)
		digits := len(date) - len(strings.TrimLeft(date, "0123456789"))
		if year, err := strconv.Atoi(date[:digits]); err == nil && year > 1900 && year <= 1970 {
			return patchDevNull
		}
	}
	return strings.TrimSpace(name)
}

// parseHunkHeader reads `@@ -a[,b] +c[,d] @@`, where a length left out is 1.
func parseHunkHeader(line string) (patchHunk, error) {
	fields := strings.Fields(line)
	malformed := fmt.Errorf("malformed hunk header: %s", line)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return patchHunk{}, malformed
	}
	oldStart, oldLen, oldErr := patchRange(fields[1][1:])
	newStart, newLen, newErr := patchRange(fields[2][1:])
	if oldErr != nil || newErr != nil || oldLen < 1 && newLen < 1 {
		return patchHunk{}, malformed
	}
	return patchHunk{oldStart: oldStart, oldLen: oldLen, newStart: newStart, newLen: newLen}, nil
}

func patchRange(field string) (int, int, error) {
	startText, lengthText, ranged := strings.Cut(field, ",")
	start, err := strconv.Atoi(startText)
	if err != nil || !ranged {
		return start, 1, err
	}
	length, err := strconv.Atoi(lengthText)
	return start, length, err
}
