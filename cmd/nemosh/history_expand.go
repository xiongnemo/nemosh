package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// History expansion: `!!`, `!n`, `!-n`, `!string`, `!$`, `!^`, `!*`, and `^old^new`.
//
// The one interactive feature a daily user reaches for most after Tab, and the largest
// hole line editing left behind: arrows, Ctrl-R and suggestions all landed, and `!!` --
// the shortest way to say "that again" -- did not.
//
// It is a **textual** rewrite done before the line is parsed, which is what bash does and
// why it is here in the session rather than in the runtime: the shell proper never sees
// an unexpanded `!`. Three rules carry it, all measured against bash rather than recalled:
//
//   - **Single quotes protect, double quotes do not.** `echo '!!'` is two characters and
//     `echo "!!"` is the previous command. That asymmetry is not a quirk to be tidied
//     away -- scripts and muscle memory both rely on it.
//   - **A backslash escapes**, so `\!` is a literal `!` and the backslash goes.
//   - **A `!` that begins nothing is itself.** `!` at end of line, before a blank, or
//     before `=` is ordinary text, which is what keeps `[ $x != y ]` working.
//
// A reference that cannot be resolved is an **error, and the line does not run**. Silently
// leaving the text as typed would send `!vim` to PATH as a command name.

// historyEvent is what a failed expansion reports.
type historyEvent struct{ text string }

func (e historyEvent) Error() string { return e.text + ": event not found" }

// expandHistory rewrites a line against the history, newest last.
//
// The second result says whether anything changed, because bash echoes the expansion
// before running it and echoing an unchanged line would be noise on every command.
func expandHistory(line string, entries []string) (string, bool, error) {
	if replaced, ok, err := quickSubstitution(line, entries); ok || err != nil {
		return replaced, ok, err
	}
	if !strings.ContainsRune(line, '!') {
		return line, false, nil
	}
	var out strings.Builder
	changed := false
	inSingle, inDouble := false, false
	for index := 0; index < len(line); index++ {
		switch character := line[index]; {
		case character == '\\' && index+1 < len(line):
			// A backslash escapes the next character. Before `!` the backslash is
			// consumed, as bash does; elsewhere both survive so a path keeps its
			// separators.
			if line[index+1] == '!' && !inSingle {
				out.WriteByte('!')
				index++
				changed = true
				continue
			}
			out.WriteByte(character)
			out.WriteByte(line[index+1])
			index++
		case character == '\'' && !inDouble:
			inSingle = !inSingle
			out.WriteByte(character)
		case character == '"' && !inSingle:
			inDouble = !inDouble
			out.WriteByte(character)
		case character == '!' && !inSingle:
			replacement, width, ok, err := historyReference(line[index:], entries)
			if err != nil {
				return "", false, err
			}
			if !ok {
				out.WriteByte(character)
				continue
			}
			out.WriteString(replacement)
			index += width - 1
			changed = true
		default:
			out.WriteByte(character)
		}
	}
	return out.String(), changed, nil
}

// historyReference reads one `!...` and answers what it means and how long it was.
//
// ok is false for a `!` that begins nothing -- at the end of a line, before a blank, or
// before `=`. Those are ordinary text, and treating them otherwise would break
// `[ "$a" != "$b" ]`, which is the first thing anyone would notice.
func historyReference(text string, entries []string) (replacement string, width int, ok bool, err error) {
	if len(text) < 2 {
		return "", 0, false, nil
	}
	switch text[1] {
	case ' ', '\t', '=', '!':
		if text[1] != '!' {
			return "", 0, false, nil
		}
		// `!!` is the previous line entire.
		previous, found := historyEntry(entries, -1)
		if !found {
			return "", 0, false, historyEvent{text: "!!"}
		}
		return previous, 2, true, nil
	case '$', '^', '*':
		previous, found := historyEntry(entries, -1)
		if !found {
			return "", 0, false, historyEvent{text: text[:2]}
		}
		return historyWords(previous, text[1]), 2, true, nil
	case '?':
		// `!?text?` is the most recent line containing text.
		end := strings.IndexByte(text[2:], '?')
		if end < 0 {
			return "", 0, false, historyEvent{text: text}
		}
		needle := text[2 : 2+end]
		for index := len(entries) - 1; index >= 0; index-- {
			if strings.Contains(entries[index], needle) {
				return entries[index], 3 + end, true, nil
			}
		}
		return "", 0, false, historyEvent{text: text[:3+end]}
	}
	// A number, a negative number, or a prefix.
	end := 1
	if text[end] == '-' {
		end++
	}
	start := end
	for end < len(text) && isHistoryWordByte(text[end]) {
		end++
	}
	if end == start {
		return "", 0, false, nil
	}
	token := text[1:end]
	if number, convErr := strconv.Atoi(token); convErr == nil {
		entry, found := historyEntry(entries, number)
		if !found {
			return "", 0, false, historyEvent{text: "!" + token}
		}
		return entry, end, true, nil
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if strings.HasPrefix(entries[index], token) {
			return entries[index], end, true, nil
		}
	}
	return "", 0, false, historyEvent{text: "!" + token}
}

// isHistoryWordByte is what may appear in a `!prefix` or a number.
//
// Deliberately narrow: a `!` reference ends at the first character that could not begin a
// command name, so `!ls|wc` finds `ls` and leaves the pipe where it was.
func isHistoryWordByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' ||
		b == '_' || b == '-' || b == '.' || b == '/'
}

// historyEntry resolves a number: positive counts from the start as `history` numbers,
// negative from the end, and -1 is the previous line.
func historyEntry(entries []string, number int) (string, bool) {
	if len(entries) == 0 {
		return "", false
	}
	index := number - 1
	if number < 0 {
		index = len(entries) + number
	}
	if index < 0 || index >= len(entries) {
		return "", false
	}
	return entries[index], true
}

// historyWords is `!$`, `!^` and `!*` on a line: its last word, its first argument, and
// all of its arguments.
//
// Split on blanks rather than by the shell's own rules, which is what bash does here --
// the designators are older than the parser and work on text.
func historyWords(line string, designator byte) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	switch designator {
	case '$':
		return words[len(words)-1]
	case '^':
		if len(words) < 2 {
			return ""
		}
		return words[1]
	default:
		if len(words) < 2 {
			return ""
		}
		return strings.Join(words[1:], " ")
	}
}

// quickSubstitution is `^old^new`, which repeats the previous line with the first
// occurrence of old replaced.
//
// Only at the very start of a line -- that is what makes it unambiguous against a `^`
// anywhere else, and it is bash's rule.
func quickSubstitution(line string, entries []string) (string, bool, error) {
	if !strings.HasPrefix(line, "^") {
		return "", false, nil
	}
	rest := line[1:]
	end := strings.IndexByte(rest, '^')
	if end < 0 {
		return "", false, nil
	}
	old := rest[:end]
	replacement := strings.TrimSuffix(rest[end+1:], "^")
	previous, found := historyEntry(entries, -1)
	if !found || !strings.Contains(previous, old) {
		return "", false, fmt.Errorf("%s: substitution failed", line)
	}
	return strings.Replace(previous, old, replacement, 1), true, nil
}

// historySource is what expansion needs from the shell, which is one list.
//
// An interface rather than the concrete Runtime so the wiring can be driven in a test
// without building a shell -- and so that the two interactive loops, which differ in
// almost everything else, demonstrably share this.
type historySource interface{ HistoryEntries() []string }

// applyHistoryExpansion rewrites a typed line and says whether it should run.
//
// Called from both interactive loops: the edited one when there is a terminal, and the
// plain one when stdin is a pipe. They were written separately and it would be easy to
// give this to only the first -- which is exactly what happened on the first attempt, and
// what made `!!` do nothing when the shell was driven by a script.
//
// The line's own terminator is put back afterwards, because the plain loop accumulates
// lines with their newlines and the edited one does not.
func applyHistoryExpansion(source historySource, stderr io.Writer, line string) (string, bool) {
	body := strings.TrimRight(line, "\r\n")
	terminator := line[len(body):]
	expanded, changed, err := expandHistory(body, source.HistoryEntries())
	if err != nil {
		// The line does not run. Leaving it as typed would send `!vim` to PATH as a
		// command name, and a shell that guesses here is worse than one that refuses.
		fmt.Fprintf(stderr, "nemosh: %v\n", err)
		return "", false
	}
	if !changed {
		return line, true
	}
	// Echoed so the user sees what will run, which is what bash does -- on stderr, so a
	// redirected stdout still holds only what the command wrote.
	fmt.Fprintln(stderr, expanded)
	return expanded + terminator, true
}
