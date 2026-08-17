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
		// `$((` before `$(`: the command substitution rule counts one closing
		// paren and would stop at the first of the two that end an arithmetic
		// expansion, leaving the second to be reported as unexpected.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && quote != '\'' {
			end, ok := arithmeticExpansionEnd(line, index+3)
			if !ok {
				return "", nil, fmt.Errorf("%w: unterminated arithmetic expansion", ErrIncompleteScript)
			}
			output.WriteString(line[index : end+1])
			index = end + 1
			continue
		}
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
		// An array assignment's parentheses are part of a word, so they are
		// stepped over whole. Third layer that has to know this -- the scanner
		// decides where a logical line ends, the lexer where a word ends, and
		// this one where a group is. Missing it here left the `)` with no opener
		// to match and reported `syntax error: unexpected )`.
		if char == '(' {
			if end, ok := arrayAssignmentSpan(line, index, output.String()); ok {
				output.WriteString(line[index : end+1])
				index = end + 1
				continue
			}
		}
		start, opener, ok := groupOpenerAt(line, index)
		if !ok {
			// A brace outside command position is an ordinary character, so
			// only one that really is the reserved word is unmatched here.
			if char == ')' || braceDelimiterAt(line, index, '}') {
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

// braceDelimiterAt reports whether the brace at index is the reserved word
// rather than an ordinary character. POSIX 2.4 makes `{` and `}` reserved only
// where a command name could start, which is why `echo a}b` prints `a}b` and
// `find . -exec rm {} \;` passes `{}` through untouched.
//
// The previous byte alone cannot decide that, because blanks separate a brace
// from whatever put it in command position. What matters is the previous
// non-blank, and the two directions do not want the same set:
//
//   - `{` opens a group after a separator, after the `(` of a subshell or the
//     `{` of an enclosing group, or after the `)` that ends a function
//     definition's parameter list, so `f(){ echo a; }` is a definition.
//   - `}` closes one only after a separator. After the `))` of an arithmetic
//     expansion the scan is still inside a word, so the first `}` in
//     `{ echo $((1+2))}; }` is text and only the second one closes.
//
// bash, dash, and busybox ash agree on every case above.
func braceDelimiterAt(line string, index int, delimiter byte) bool {
	if line[index] != delimiter {
		return false
	}
	if index+1 != len(line) && !isCommandBoundary(line[index+1]) {
		return false
	}
	previous, found := previousNonBlank(line, index)
	if !found || isCommandSeparator(previous) {
		return true
	}
	return delimiter == '{' && (previous == ')' || previous == '(' || previous == '{')
}

// previousNonBlank reports the last character before index that is not a blank,
// and whether the scan found one before running off the front of the line.
func previousNonBlank(line string, index int) (byte, bool) {
	for back := index - 1; back >= 0; back-- {
		if line[back] != ' ' && line[back] != '\t' {
			return line[back], true
		}
	}
	return 0, false
}

// isCommandSeparator reports whether a character ends one command and so leaves
// the next byte in command position.
func isCommandSeparator(char byte) bool {
	return char == ';' || char == '&' || char == '|' || char == '\n'
}

func matchingGroupEnd(line string, start int, opener byte) (int, error) {
	closers := []byte{closerFor(opener)}
	quote := byte(0)
	escaped := false
	for index := start + 1; index < len(line); index++ {
		char := line[index]
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && quote != '\'' {
			end, ok := arithmeticExpansionEnd(line, index+3)
			if !ok {
				return 0, fmt.Errorf("%w: unterminated arithmetic expansion", ErrIncompleteScript)
			}
			index = end
			continue
		}
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
