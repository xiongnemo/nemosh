package runtime

import (
	"context"
	"maps"
	"strings"
)

// What a text read in bash mode does when the aliases it was read with change as it runs: bash
// reads a command, runs it and reads the next, so `alias x=...` on one line is in force for x on
// the next, and on its own line is not. nemosh reads the text whole, so it runs what it read a line
// at a time and, when the aliases have changed, reads the rest again from the next command -- when
// a command ends before it, and the change is one the rest would read differently. A `.bashrc`
// defining forty aliases is so read once, unless it goes on to use them.

// runRead runs a program read from text a line at a time, as executeStatements runs one, and
// answers the line to read the rest of text again from, or 0 to go on past text.end.
func (r Runtime) runRead(ctx context.Context, program []programNode, status int, topLevel bool, text *readText) (int, flowControl, int) {
	boundary := text.start
	for from := 0; from < len(program); {
		to := lineGroupEnd(program, from)
		var control flowControl
		status, control = r.executeStatements(ctx, program[from:to], status, topLevel, true)
		if control != flowNone || r.lifecycle.exitSuppressed || ctx.Err() != nil {
			return status, control, 0
		}
		ran := statementLine(program[from])
		from = to
		if from < len(program) && r.aliasReading() != text.print {
			if line, again := r.rereadAt(text, ran-text.first, statementLine(program[from])-text.first, &boundary); again {
				return status, flowNone, line
			}
		}
	}
	return status, flowNone, 0
}

// lineGroupEnd is the index past the statements that begin on the line program[from] does.
func lineGroupEnd(program []programNode, from int) int {
	line := statementLine(program[from])
	to := from + 1
	for to < len(program) && (statementLine(program[to]) == line || statementLine(program[to]) == 0) {
		to++
	}
	return to
}

// rereadAt is the line to read the rest of text again from, the statements on line ran having
// run and the next being on line next: the line the command just run ends before, when it ends
// before next and the aliases now read the lines from there otherwise than those text was read
// with. boundary is the last line known to begin a command, and the end is the first line after
// ran that what lies from boundary to it parses whole at; none up to next, and next is part of
// the command just run, as in `fi; x`. Asked line by line, as a statement's line is its first
// command's and a function's or a group's opens before it, but only when the aliases changed. A
// change the rest does not read differently is taken as what text was read with, and the run
// goes on.
func (r Runtime) rereadAt(text *readText, ran, next int, boundary *int) (int, bool) {
	if ran < *boundary || next <= ran || next >= text.end {
		return 0, false
	}
	end := -1
	for line := ran + 1; line <= next && end < 0; line++ {
		if _, err := text.parse(*boundary, line); err == nil {
			end = line
		}
	}
	if end < 0 {
		return 0, false
	}
	*boundary = end
	now := r.readingAliases()
	if readsDifferently(text.reading, now, text.lines[end:text.end]) {
		return end, true
	}
	text.reading, text.print = maps.Clone(now), r.aliasReading()
	return 0, false
}

// readsDifferently is whether lines read with the aliases now would read otherwise than with old:
// when an alias added, removed or changed between them is written in them as a command's name
// could be, or in any alias's text, which reading them could bring in.
func readsDifferently(old, now map[string]string, lines []string) bool {
	for _, name := range changedAliases(old, now) {
		for _, aliases := range [...]map[string]string{old, now} {
			for _, value := range aliases {
				if strings.Contains(value, name) {
					return true
				}
			}
		}
		for _, line := range lines {
			if containsCommandWord(line, name) {
				return true
			}
		}
	}
	return false
}

// changedAliases is the names that old and now define differently, or only one of them defines.
func changedAliases(old, now map[string]string) []string {
	var names []string
	for name, value := range now {
		if before, ok := old[name]; !ok || before != value {
			names = append(names, name)
		}
	}
	for name := range old {
		if _, ok := now[name]; !ok {
			names = append(names, name)
		}
	}
	return names
}

// containsCommandWord is whether name is written in line as a word of its own, between blanks,
// operators and the line's ends, where it could be read as an alias. It errs towards yes.
func containsCommandWord(line, name string) bool {
	for offset := 0; name != ""; {
		found := strings.Index(line[offset:], name)
		if found < 0 {
			return false
		}
		at, end := offset+found, offset+found+len(name)
		before := at == 0 || strings.IndexByte(" \t\n;&|(`", line[at-1]) >= 0
		after := end == len(line) || strings.IndexByte(" \t\r\n;&|)<>`", line[end]) >= 0
		if before && after {
			return true
		}
		offset = at + 1
	}
	return false
}
