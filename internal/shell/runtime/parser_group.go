package runtime

import (
	"fmt"
	"strings"
)

type parsedGroup struct {
	brace bool
	body  Script
}

func (g parsedGroup) withRedirects(redirects []redirectOperation) commandNode {
	if g.brace {
		return braceGroup{body: g.body, redirects: redirects}
	}
	return subshellCommand{body: g.body, redirects: redirects}
}

func extractGroupCommands(line string, budget *parseBudget, depth int) (string, map[int]parsedGroup, error) {
	groups := make(map[int]parsedGroup)
	var output strings.Builder
	quote := byte(0)
	escaped := false
	for index := 0; index < len(line); {
		char := line[index]
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && quote != '\'' {
			end, ok := commandSubstitutionEnd(line, index+2)
			if !ok {
				return "", nil, fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
			}
			output.WriteString(line[index : end+1])
			index = end + 1
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '{' && quote != '\'' {
			end := strings.IndexByte(line[index+2:], '}')
			if end < 0 {
				output.WriteString(line[index:])
				break
			}
			end += index + 2
			output.WriteString(line[index : end+1])
			index = end + 1
			continue
		}
		if escaped {
			output.WriteByte(char)
			escaped = false
			index++
			continue
		}
		if char == '\\' && quote != '\'' {
			output.WriteByte(char)
			escaped = true
			index++
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			output.WriteByte(char)
			index++
			continue
		}
		if quote != 0 {
			output.WriteByte(char)
			index++
			continue
		}
		start, opener, ok := groupOpenerAt(line, index)
		if !ok {
			if char == '}' || char == ')' {
				return "", nil, fmt.Errorf("syntax error: unexpected %c", char)
			}
			output.WriteByte(char)
			index++
			continue
		}
		end, err := matchingGroupEnd(line, start, opener)
		if err != nil {
			return "", nil, err
		}
		body := line[start+1 : end]
		if opener == '{' && !hasBraceSeparator(body) {
			return "", nil, fmt.Errorf("syntax error: expected separator before }")
		}
		body = normalizeGroupSeparators(body)
		nested, err := parseScript(body, budget, depth+1)
		if err != nil {
			return "", nil, err
		}
		groups[output.Len()] = parsedGroup{brace: opener == '{', body: nested}
		output.WriteString("__nemosh_group__")
		index = end + 1
	}
	return output.String(), groups, nil
}

func scanExtractedGroups(line string, groups map[int]parsedGroup, budget *parseBudget, depth int) ([]shellToken, error) {
	tokens, starts, err := scanShellTokensWithPositions(line, budget, depth)
	if err != nil {
		return nil, err
	}
	for index := range tokens {
		group, ok := groups[starts[index]]
		if ok && tokens[index].kind == tokenWord {
			tokens[index].group = &group
		}
	}
	return tokens, nil
}

func groupOpenerAt(line string, index int) (int, byte, bool) {
	if line[index] != '{' && line[index] != '(' {
		return 0, 0, false
	}
	if line[index] == '{' && !braceDelimiterAt(line, index, '{') {
		return 0, 0, false
	}
	if line[index] == '(' && index > 0 && (!isCommandBoundary(line[index-1]) || line[index-1] == '$') {
		return 0, 0, false
	}
	return index, line[index], true
}

func isCommandBoundary(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '|' || char == '&' || char == '(' || char == '{' || char == ';'
}

func braceDelimiterAt(line string, index int, delimiter byte) bool {
	if line[index] != delimiter {
		return false
	}
	if index > 0 && !isCommandBoundary(line[index-1]) {
		return false
	}
	return index+1 == len(line) || isCommandBoundary(line[index+1])
}

func matchingGroupEnd(line string, start int, opener byte) (int, error) {
	closers := []byte{closerFor(opener)}
	quote := byte(0)
	escaped := false
	for index := start + 1; index < len(line); index++ {
		char := line[index]
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && quote != '\'' {
			end, ok := commandSubstitutionEnd(line, index+2)
			if !ok {
				return 0, fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
			}
			index = end
			continue
		}
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
		if quote != 0 {
			continue
		}
		if char == '(' || braceDelimiterAt(line, index, '{') {
			if len(closers) >= maxParseDepth {
				return 0, fmt.Errorf("group depth: %w", errParseLimit)
			}
			closers = append(closers, closerFor(char))
			continue
		}
		if char != ')' && !braceDelimiterAt(line, index, '}') {
			continue
		}
		expected := closers[len(closers)-1]
		if char != expected {
			return 0, fmt.Errorf("syntax error: unexpected %c, expected %c", char, expected)
		}
		closers = closers[:len(closers)-1]
		if len(closers) == 0 {
			return index, nil
		}
	}
	return 0, fmt.Errorf("%w: missing %c", ErrIncompleteScript, closers[len(closers)-1])
}

func closerFor(opener byte) byte {
	if opener == '{' {
		return '}'
	}
	return ')'
}

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
