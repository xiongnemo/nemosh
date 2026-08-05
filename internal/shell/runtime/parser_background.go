package runtime

import "strings"

func trailingBackground(line string) (string, bool) {
	trimmed := strings.TrimRight(line, " \t")
	if len(trimmed) == 0 || trimmed[len(trimmed)-1] != '&' || len(trimmed) > 1 && (trimmed[len(trimmed)-2] == '&' || trimmed[len(trimmed)-2] == '<' || trimmed[len(trimmed)-2] == '>') {
		return line, false
	}
	quote := byte(0)
	escaped := false
	for index := 0; index < len(trimmed); index++ {
		char := trimmed[index]
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
		if index == len(trimmed)-1 && quote == 0 {
			return strings.TrimSpace(trimmed[:index]), true
		}
	}
	return line, false
}
