package runtime

import "strings"

// expandElifLines rewrites `elif C` into `else` followed by a nested `if C`,
// which is what the construct means -- POSIX 2.9.4.1 defines the elif chain as
// exactly that nesting. Each rewrite owes one extra `fi`, paid when the real
// one arrives.
//
// Doing it here rather than in compoundSpans keeps one condition and one then
// per if-span. The alternative was a list of condition/then pairs threaded
// through the span, the typed node, and execution, for a construct that has no
// semantics of its own.
//
// Only `if` frames can owe anything, but every compound opener is tracked so
// the `fi` that closes an if is not confused with the `done` that closes a loop
// nested inside its body. An opener counts after other words, as in `true && if`, and a
// closer with words after it, as in `fi > log` and `fi | while read l`, which closes one
// compound and opens the next. Only a line that was exactly an opener or a closer counted,
// so the elif chain in `true && if ...` paid its `fi` to the wrong frame ("duplicate
// then"), and the one in `if ...; elif ...; fi | cat` never paid it ("missing fi").
func expandElifLines(lines []string, at []int) ([]string, []int) {
	var expanded []string
	var expandedAt []int
	var owed []int
	for index, line := range lines {
		start := startOf(at, index)
		header := line
		if prefix, operator, rest, ok := splitCompoundAfterPrefix(line); ok {
			if closer, _, closes := chainedCloser(prefix, operator); closes {
				expanded, expandedAt, owed = payOwedClosers(expanded, expandedAt, owed, closer, start)
			}
			header = rest
		}
		// A keyword alone on its line opens its compound too, its condition on the lines after
		// it: `if`, `elif` and the loops. Neither bare `if` nor bare `elif` counted, so `if` with
		// its condition below, or `...; elif` with its below, was `duplicate then`.
		bare, alone := bareConditionOpener(header)
		switch {
		case hasCompoundHeader(header, "if"), alone && bare == compoundIf:
			owed = append(owed, 0)
		case hasCompoundHeader(header, "for"), hasCompoundHeader(header, "while"),
			hasCompoundHeader(header, "until"), hasCompoundHeader(header, "case"),
			hasCompoundHeader(header, "select"), alone:
			// Not an if, so it can never owe an extra closer; the -1 marks it.
			owed = append(owed, -1)
		default:
			if closer, ok := compoundCloserWord(line); ok {
				// The extra closers come first, so what follows this one stays on the
				// outermost if.
				expanded, expandedAt, owed = payOwedClosers(expanded, expandedAt, owed, closer, start)
				break
			}
			condition, ok := compoundHeader(line, "elif")
			if line == "elif" {
				condition, ok = "", true
			}
			if !ok || len(owed) == 0 || owed[len(owed)-1] < 0 {
				break
			}
			owed[len(owed)-1]++
			expanded, expandedAt = appendNumbered(expanded, expandedAt, "else", start)
			expanded, expandedAt = appendNumbered(expanded, expandedAt, strings.TrimSpace("if "+condition), start)
			continue
		}
		expanded, expandedAt = appendNumbered(expanded, expandedAt, line, start)
	}
	return expanded, expandedAt
}

// payOwedClosers closes the innermost compound's frame, adding the closers its elif chain owes.
func payOwedClosers(expanded []string, at []int, owed []int, closer string, start int) ([]string, []int, []int) {
	if len(owed) == 0 {
		return expanded, at, owed
	}
	for range max(owed[len(owed)-1], 0) {
		expanded, at = appendNumbered(expanded, at, closer, start)
	}
	return expanded, at, owed[:len(owed)-1]
}

// compoundCloserWord is the closer a line is, alone or with words after it.
func compoundCloserWord(line string) (string, bool) {
	if line == "fi" || line == "done" || line == "esac" {
		return line, true
	}
	closer, _, ok := splitCompoundCloser(line)
	return closer, ok
}
