package runtime

import "strings"

// arithmeticSpanAt is the expression of the arithmetic expansion at index and its last byte: a
// `$((...))`, and in bash mode bash's older spelling of one, `$[...]`, which busybox leaves as
// text and nemosh and sh with it. Measured with Git for Windows' bash 5.3.
func arithmeticSpanAt(text string, index int, bash bool) (string, int, bool) {
	if strings.HasPrefix(text[index:], "$((") {
		if end, ok := arithmeticExpansionEnd(text, index+3); ok {
			return text[index+3 : end-1], end, true
		}
	}
	if end, ok := legacyArithmeticEnd(text, index); ok && bash {
		return text[index+2 : end], end, true
	}
	return "", 0, false
}

// respellLegacyArithmetic writes each `$[expr]` in a script read in bash mode as `$((expr))`,
// once its here-document bodies are out, so that every scan of it reads the arithmetic as such:
// to the one that finds groups, the `(` of `$[(1+2)*3]` was a subshell's, and to the heredoc scan
// the `<<` of `$[1<<3]` a here-document's. Not in single quotes, `$'...'`, a comment or after a
// backslash, where bash leaves it as text; open lists the quotes and the `$(` it is inside,
// innermost last. declare -f lists such a function with `$((...))`, and bash with what was written.
func respellLegacyArithmetic(text string) string {
	if !strings.Contains(text, "$[") {
		return text
	}
	var out strings.Builder
	var open []byte
	for index := 0; index < len(text); index++ {
		char, inside := text[index], byte(0)
		if len(open) > 0 {
			inside = open[len(open)-1]
		}
		switch {
		case inside == '\'':
			if char == '\'' {
				open = open[:len(open)-1]
			}
		case char == '\\' && index+1 < len(text):
			out.WriteByte(char)
			index++
			char = text[index]
		case char == '"' && inside == '"', char == ')' && inside == '(':
			open = open[:len(open)-1]
		case char == '"', char == '\'' && inside != '"', char == '(' && inside == '(':
			open = append(open, char)
		case char == '#' && inside != '"' && commentStarts(text, index):
			end := strings.IndexByte(text[index:], '\n')
			if end < 0 {
				end = len(text) - index
			}
			out.WriteString(text[index : index+end])
			index += end - 1
			continue
		case char == '$':
			if end := ansiQuoteClose(text, index); end >= 0 {
				out.WriteString(text[index : end+1])
				index = end
				continue
			}
			// One opened in double quotes closes before they do: `"$[" "]"` is no arithmetic.
			if end, ok := legacyArithmeticEnd(text, index); ok && (inside != '"' || !strings.Contains(text[index:end], `"`)) {
				out.WriteString("$((" + respellLegacyArithmetic(text[index+2:end]) + "))")
				index = end
				continue
			}
			if index+1 < len(text) && text[index+1] == '(' {
				open = append(open, '(')
				out.WriteString("$(")
				index++
				continue
			}
		}
		out.WriteByte(char)
	}
	return out.String()
}

// legacyArithmeticEnd is the `]` that closes the `$[` at index, the brackets in it nesting as
// bash's do, `$[a[1]+1]`; false where none begins or none closes.
func legacyArithmeticEnd(text string, index int) (int, bool) {
	if !strings.HasPrefix(text[index:], "$[") {
		return 0, false
	}
	depth := 0
	for at := index + 1; at < len(text); at++ {
		switch text[at] {
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				return at, true
			}
		}
	}
	return 0, false
}
