package runtime

import "strings"

// The line a command inside a list starts on, when the list goes on past its first line:
// `a &&` with b on the next, `a \` with `| b` on the next, or `} && c` after a group that
// spans lines. Both references give b and c their own lines. Here every command of a parser
// line had the line's first, since the joins had left no trace of where each later physical
// line began.
//
// The scanner knows where each did, as it joins them. So each parser line carries those
// offsets in its text (breaksWithin), through the passes that reshape lines (carryBreaks),
// to the tokens, which know their own offsets. A command's line is its parser line's plus
// the breaks before its first word. A line a pass made anew has none recorded, and all of
// its commands count from its start, as they all did.

// breaksWithin is where, in the text of length size at offset in the logical line, a later
// physical line begins, as offsets in that text.
func (scanner *syntaxScanner) breaksWithin(offset, size int) []int {
	var within []int
	for _, at := range scanner.breaks {
		if at > offset && at < offset+size {
			within = append(within, at-offset)
		}
	}
	return within
}

// breakCarryWindow is how far ahead of the last line matched carryBreaks looks for a line.
// The passes split and join lines, and move none far.
const breakCarryWindow = 16

// carryBreaks gives each line the passes left the breaks it had before them, where it is one
// of those lines, unchanged and starting where it did.
func carryBreaks(before []string, beforeAt []int, breaks [][]int, after []string, afterAt []int) [][]int {
	carried := make([][]int, len(after))
	next := 0
	for index, line := range after {
		for probe := next; probe < len(before) && probe < next+breakCarryWindow; probe++ {
			if probe < len(breaks) && before[probe] == line && startOf(beforeAt, probe) == startOf(afterAt, index) {
				carried[index], next = breaks[probe], probe+1
				break
			}
		}
	}
	return carried
}

// commandLine is the source line of a command whose first token is tokens[0], read from
// readText: the line being built, plus the breaks before the token when readText is that
// line's own text, from its start.
func (budget *parseBudget) commandLine(tokens []shellToken) int {
	numbering := budget.numbering
	if len(tokens) == 0 || numbering.currentText == "" || !strings.HasPrefix(numbering.currentText, numbering.readText) {
		return budget.line()
	}
	offset := tokens[0].offset
	later := 0
	for _, at := range numbering.currentBreaks {
		if at <= offset {
			later++
		}
	}
	// Counted in the lines left once heredoc bodies were taken out, as current is.
	return numbering.number(numbering.current + later)
}

// groupPlaceholder stands for a group extractGroupCommands has taken out of a line and parsed.
const groupPlaceholder = "__nemosh_group__"

// unmaskedOffset is where offset in a line with its groups taken out is in the line as it was.
func unmaskedOffset(offset int, groups map[int]parsedGroup) int {
	shift := 0
	for at, group := range groups {
		if at < offset {
			shift += group.width - len(groupPlaceholder)
		}
	}
	return offset + shift
}

// linesBefore is how many later physical lines begin in before, the start of the text of the
// line being built: its breaks when before is that text's own start, which count the lines a
// join left no newline for, and the newlines in it otherwise. `{ a; } &&` with `{ b; }` on the
// next line put b's group on the first.
func (n lineNumbering) linesBefore(before string) int {
	if n.currentText == "" || !strings.HasPrefix(n.currentText, before) {
		return strings.Count(before, "\n")
	}
	count := 0
	for _, at := range n.currentBreaks {
		if at < len(before) {
			count++
		}
	}
	return count
}
