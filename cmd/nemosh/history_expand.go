package main

import (
	"fmt"
	"io"
	"strings"
)

// History expansion, bash's: lib/readline/histexpand.c under the settings bashhist.c gives
// it, ported rather than recalled. busybox has none.
//
// It is a **textual** rewrite done before the line is parsed, which is why it is here in the
// session rather than in the runtime: the shell proper never sees an unexpanded `!`.
//
//   - An event: `!!`, `!n`, `!-n`, `!string`, `!?string?`, and `!#`, the line typed so far.
//   - A word designator after a `:`, or after none when it begins with ^ $ * - or %: `0`,
//     `n`, `^`, `$`, `x-y`, `x*`, `x-`, `*`, and `%`, the word the last `!?string?` matched.
//     The words are the shell's -- quotes kept, operators apart -- not the blanks'.
//   - Modifiers, each after a `:`: h t r e, p to print the line and not run it, q x to quote,
//     s/old/new/ with & for old, & to repeat it, and g a G to repeat it along the line.
//   - `^old^new^` at the start of a line is `!!:s^old^new^`.
//
// **Single quotes protect, double quotes do not**, a backslash protects the `!` after it, and
// a `!` before a blank, `=`, an operator or the end of the line is itself. So is one bash's
// shell uses: `$!`, `${!name}`, `[!...]`. A `#` that begins a word ends expansion.
//
// A reference that cannot be resolved is an **error, and the line does not run**.

// historyExpander carries what bash's expansion keeps from one line to the next: the last
// substitution, which `:&` repeats and an empty old takes, and the last `!?string?` search,
// which an empty one repeats and whose matched word `%` is.
type historyExpander struct {
	lhs, rhs      string
	search, match string
}

// expandHistory is a line expanded against entries, oldest first, by an expander that
// remembers nothing: for the line editor, which asks of one word as it is typed.
func expandHistory(line string, entries []string) (string, bool, error) {
	var expander historyExpander
	expanded, changed, _, err := expander.expand(line, entries)
	return expanded, changed, err
}

// expand is history_expand: the line rewritten; whether an expansion took place, a
// backslash that kept a `!` from being one being none; and whether a :p asked for it to be
// printed and not run.
func (x *historyExpander) expand(line string, entries []string) (string, bool, bool, error) {
	text := line
	if strings.HasPrefix(text, "^") {
		text = "!!:s" + text
	} else if !historyExpansionIn(text) {
		return line, false, false, nil
	}
	var out strings.Builder
	dquote, passNext, modified, printOnly := false, false, false, false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if passNext {
			passNext = false
			out.WriteByte(c)
			continue
		}
		switch {
		case c == '\\':
			passNext = true
			out.WriteByte(c)
		case c == '"':
			dquote = !dquote
			out.WriteByte(c)
		case c == '\'' && !dquote:
			end := singleQuoteEnd(text, i+1, i > 0 && text[i-1] == '$')
			out.WriteString(text[i:min(end+1, len(text))])
			i = end
		case c == '#' && !dquote && (i == 0 || byteIn(historyWordDelimiters, text[i-1])):
			out.WriteString(text[i:])
			i = len(text)
		case c == '!':
			var next byte
			if i+1 < len(text) {
				next = text[i+1]
			}
			if next == 0 || byteIn(historyNoExpandChars, next) || dquote && next == '"' ||
				historyInhibited(out.String()+"!"+string([]byte{next}), out.Len()) {
				out.WriteByte(c)
				continue
			}
			replacement, end, printed, err := x.expandOne(text, i, dquote, out.String(), entries)
			if err != nil {
				return "", false, false, err
			}
			out.WriteString(replacement)
			modified, printOnly, i = true, printOnly || printed, end
		default:
			out.WriteByte(c)
		}
	}
	if !modified {
		return line, false, false, nil
	}
	return out.String(), true, printOnly, nil
}

// historyExpansionIn is history_expand's first pass: whether the line has a `!` expansion
// would take at all. It asks the inhibiting function of the line as typed, where the second
// pass asks it of the line as expanded so far, as bash's two passes do.
func historyExpansionIn(text string) bool {
	dquote := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		var next byte
		if i+1 < len(text) {
			next = text[i+1]
		}
		switch {
		case c == '#' && !dquote && (i == 0 || byteIn(historyWordDelimiters, text[i-1])):
			return false
		case c == '!':
			if next != 0 && !byteIn(historyNoExpandChars, next) && !(dquote && next == '"') && !historyInhibited(text, i) {
				return true
			}
		case dquote && c == '\\' && next == '"':
			i++
		case c == '"':
			dquote = !dquote
		case c == '\'' && !dquote:
			if i = singleQuoteEnd(text, i+1, i > 0 && text[i-1] == '$'); i >= len(text) {
				return false
			}
		case c == '\\' && (next == '\'' || next == '!'):
			i++
		}
	}
	return false
}

// historyInhibited is bash_history_inhibit_expansion: whether the `!` at text[i] is one the
// shell itself uses -- `[!`, `${!`, `$!` -- or is quoted where readline cannot tell, inside a
// $(...) or `...` that is itself inside double quotes.
func historyInhibited(text string, i int) bool {
	switch {
	case i > 0 && text[i-1] == '[' && strings.IndexByte(text[i+1:], ']') >= 0:
		return true
	case i > 1 && text[i-1] == '{' && text[i-2] == '$' && strings.IndexByte(text[i+1:], '}') >= 0:
		return true
	case i > 1 && text[i-1] == '$':
		return true
	}
	visible := skipToHistoryExpansion(text, 0)
	if visible <= 0 {
		return false
	}
	for visible < i {
		if visible = skipToHistoryExpansion(text, visible+1); visible <= 0 {
			return false
		}
	}
	return visible > i
}

// skipToHistoryExpansion is skip_to_histexp: the index of the first `!` at or after start
// that no quoting hides, the shell's quoting rather than readline's, or the text's end.
func skipToHistoryExpansion(s string, start int) int {
	passNext, backquote, dquote, outerDquote := false, false, false, false
	substitutions := 0
	for i := start; i < len(s); {
		c := s[i]
		var next byte
		if i+1 < len(s) {
			next = s[i+1]
		}
		switch {
		case passNext:
			passNext = false
		case c == '\\':
			passNext = true
		case backquote && c == '`':
			backquote, dquote = false, outerDquote
		case c == '`':
			backquote, outerDquote, dquote = true, dquote, false
		case dquote && c == '!' && next == '"':
		case c == '!':
			return i
		case dquote && c == '\'':
		case c == '\'':
			i = singleQuoteEnd(s, i+1, false) + 1
			continue
		case c == '"':
			dquote = !dquote
		case (c == '$' || c == '<' || c == '>') && next == '(' && (i+2 >= len(s) || s[i+2] != '('):
			if i+2 >= len(s) {
				return i + 2
			}
			i += 2
			substitutions++
			outerDquote, dquote = dquote, false
			continue
		case substitutions > 0 && c == ')':
			substitutions--
			dquote = outerDquote
		}
		i++
	}
	return len(s)
}

// historySource is what expansion needs from the shell, which is one list.
//
// An interface rather than the concrete Runtime so the wiring can be driven in a test
// without building a shell -- and so that the two interactive loops, which differ in
// almost everything else, demonstrably share this.
type historySource interface {
	HistoryEntries() []string
	HistoryExpansion() bool
}

// historyOutcome is what a typed line comes to after expansion.
type historyOutcome int

const (
	// historyRun: the line runs, rewritten or not.
	historyRun historyOutcome = iota
	// historyPrinted: a :p printed it, and it goes into the history without running.
	historyPrinted
	// historyRefused: an expansion failed, and it neither runs nor is recorded.
	historyRefused
)

// applyHistoryExpansion rewrites a typed line and says what becomes of it.
//
// Called from both interactive loops: the edited one when there is a terminal, and the
// plain one when stdin is a pipe. They were written separately and it would be easy to
// give this to only the first -- which is exactly what happened on the first attempt, and
// what made `!!` do nothing when the shell was driven by a script.
//
// The line's own terminator is put back afterwards, because the plain loop accumulates
// lines with their newlines and the edited one does not.
func applyHistoryExpansion(source historySource, expander *historyExpander, stderr io.Writer, line string) (string, historyOutcome) {
	// `set +H` turns it off, as in bash.
	if !source.HistoryExpansion() {
		return line, historyRun
	}
	body := strings.TrimRight(line, "\r\n")
	terminator := line[len(body):]
	expanded, changed, printOnly, err := expander.expand(body, source.HistoryEntries())
	if err != nil {
		// The line does not run. Leaving it as typed would send `!vim` to PATH as a
		// command name, and a shell that guesses here is worse than one that refuses.
		fmt.Fprintf(stderr, "nemosh: %v\n", err)
		return "", historyRefused
	}
	if !changed {
		return line, historyRun
	}
	// Echoed so the user sees what will run, which is what bash does -- on stderr, so a
	// redirected stdout still holds only what the command wrote.
	fmt.Fprintln(stderr, expanded)
	if printOnly {
		return expanded, historyPrinted
	}
	return expanded + terminator, historyRun
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
