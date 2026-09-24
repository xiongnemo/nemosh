package runtime

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
// nested inside its body.
func expandElifLines(lines []string, at []int) ([]string, []int) {
	var expanded []string
	var expandedAt []int
	var owed []int
	for index, line := range lines {
		start := startOf(at, index)
		switch {
		case hasCompoundHeader(line, "if"):
			owed = append(owed, 0)
		case hasCompoundHeader(line, "for"), hasCompoundHeader(line, "while"),
			hasCompoundHeader(line, "until"), hasCompoundHeader(line, "case"),
			hasCompoundHeader(line, "select"):
			// Not an if, so it can never owe an extra closer; the -1 marks it.
			owed = append(owed, -1)
		case line == "fi" || line == "done" || line == "esac":
			extra := 0
			if len(owed) > 0 {
				extra = max(owed[len(owed)-1], 0)
				owed = owed[:len(owed)-1]
			}
			for range extra {
				expanded, expandedAt = appendNumbered(expanded, expandedAt, line, start)
			}
		default:
			condition, ok := compoundHeader(line, "elif")
			if !ok || len(owed) == 0 || owed[len(owed)-1] < 0 {
				break
			}
			owed[len(owed)-1]++
			expanded, expandedAt = appendNumbered(expanded, expandedAt, "else", start)
			expanded, expandedAt = appendNumbered(expanded, expandedAt, "if "+condition, start)
			continue
		}
		expanded, expandedAt = appendNumbered(expanded, expandedAt, line, start)
	}
	return expanded, expandedAt
}
