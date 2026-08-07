package runtime

import "strings"

// A brace group's body is separated from its closing brace by a `;` or a
// newline, and its own separators become newlines before it is parsed as a
// script in its own right. Split out of parser_group.go to keep that file
// under the 250-line production ceiling.

func hasBraceSeparator(body string) bool {
	trimmed := strings.TrimRight(body, " \t")
	if strings.HasSuffix(trimmed, "\n") {
		return true
	}
	if !strings.HasSuffix(trimmed, ";") {
		return false
	}
	return separatorPositions(trimmed)[len(trimmed)-1]
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
	for index := range len(body) {
		char := body[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
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
