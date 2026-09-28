package runtime

import "strings"

// A brace group's body is separated from its closing brace by a `;` or a
// newline, and its own separators become newlines before it is parsed as a
// script in its own right. Split out of parser_group.go to keep that file
// under the 250-line production ceiling.

func hasBraceSeparator(body string) bool {
	trimmed := strings.TrimRight(body, " \t")
	// A background `&` ends the last command as `;` does: `{ sleep 1 & }` in both
	// references.
	if strings.HasSuffix(trimmed, "\n") || endsWithBackgroundOperator(trimmed) {
		return true
	}
	if !strings.HasSuffix(trimmed, ";") {
		return endsWithCompound(trimmed)
	}
	return separatorPositions(trimmed)[len(trimmed)-1]
}

// endsWithCompound reports a body whose last command is a compound one -- a group, a
// subshell, an if, a loop, a case -- after which the `}` stands where a reserved word may, so
// it needs no separator, as POSIX has it and both references read it: `{ { echo in; } }` and
// `{ if x; then y; fi }` were "expected separator before }". After a simple command the `}`
// is an argument still, so `{ echo $(echo s) }` is refused, as in both.
func endsWithCompound(body string) bool {
	segments, err := splitSequentialSegments(body)
	if err != nil || len(segments) == 0 {
		return false
	}
	last := strings.TrimSpace(segments[len(segments)-1])
	switch {
	case last == "fi" || last == "done" || last == "esac":
		return true
	case strings.HasPrefix(last, "(") && strings.HasSuffix(last, ")"):
		return true
	}
	return strings.HasSuffix(last, "}") && braceDelimiterAt(last, len(last)-1, '}')
}

func normalizeGroupSeparators(body string) string {
	separators := separatorPositions(body)
	var normalized strings.Builder
	for index := range len(body) {
		if separators[index] {
			normalized.WriteByte('\n')
		} else {
			normalized.WriteByte(body[index])
		}
	}
	return strings.TrimSpace(normalized.String())
}

func separatorPositions(body string) map[int]bool {
	positions := make(map[int]bool)
	quote := byte(0)
	escaped := false
	for index := 0; index < len(body); index++ {
		char := body[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if end := quotedSpanEnd(body, index, quote); end >= 0 {
			index = end
			continue
		}
		// An expansion's `;` is its own: `${s%;}` in a function was cut into `${s%` and `}`,
		// "missing '}'".
		if end := patternExpansionEnd(body, index); quote == 0 && end > 0 {
			index = end
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			continue
		}
		if char == ';' && quote == 0 {
			positions[index] = true
		}
	}
	return positions
}
