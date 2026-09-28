package runtime

import "strings"

// quoteAssignmentSubscripts quotes an assignment's subscript that holds blanks or operators,
// where a command begins: `a[1 + 1]=x` becomes `a["1 + 1"]=x`. bash's lexer reads a `name[`
// there to its `]`, so `a[1 + 1]=x`, `a[5&3]=x` and `a[(1+2)*3]=9` are assignments. The passes
// here cut a line at those blanks and operators before any word is read, so each ran as
// commands called `a[1` and `1]=x`. A subscript loses its quotes, in bash as here, so the quoted
// one is the same assignment. Only a subscript with no quote or backslash of its own is
// rewritten, and only at a command's start or after another assignment, as bash reads it: an
// argument, as in `echo a[1 + 1]=x`, is three words there too, and so is declare's.
func quoteAssignmentSubscripts(text string) string {
	if !strings.Contains(text, "[") {
		return text
	}
	var out strings.Builder
	start := true
	for index := 0; index < len(text); {
		char := text[index]
		switch {
		case char == ' ' || char == '\t':
			out.WriteByte(char)
			index++
			continue
		case char == '#':
			// A word begins here, so this is a comment, to the end of its line.
			end := strings.IndexByte(text[index:], '\n')
			if end < 0 {
				end = len(text) - index
			}
			out.WriteString(text[index : index+end])
			index += end
			continue
		case char == '(' && strings.HasPrefix(text[index:], "(("):
			// An arithmetic command is an expression, and no command begins in it.
			if end := arithmeticCommandEnd(text, index); end > 0 {
				out.WriteString(text[index : end+1])
				index, start = end+1, false
				continue
			}
			fallthrough
		case char == '\n' || strings.IndexByte(";&|()", char) >= 0:
			out.WriteByte(char)
			index, start = index+1, true
			continue
		}
		if start {
			if quoted, end, ok := quotedSubscript(text, index); ok {
				out.WriteString(quoted)
				// The value is the rest of the word, and another assignment may follow it.
				valueEnd, ok := shellWordEnd(text, end)
				if !ok {
					return text
				}
				out.WriteString(text[end:valueEnd])
				index = valueEnd
				continue
			}
		}
		end, ok := shellWordEnd(text, index)
		if !ok {
			return text
		}
		word := text[index:end]
		out.WriteString(word)
		index = end
		switch {
		case strings.HasSuffix(word, "=") && index < len(text) && text[index] == '(':
			// An array literal's elements are no command's start.
			close, found := matchingParenthesis(text, index)
			if !found {
				return text
			}
			out.WriteString(text[index : close+1])
			index = close + 1
			continue
		case start && word == "function":
			// Its name is no command, and the body after it begins one: `function f { ...; }`.
			for index < len(text) && (text[index] == ' ' || text[index] == '\t') {
				out.WriteByte(text[index])
				index++
			}
			if end, ok = shellWordEnd(text, index); !ok {
				return text
			}
			out.WriteString(text[index:end])
			index = end
			continue
		}
		start = start && (isAssignment(word) || beginsCommand[word])
	}
	return out.String()
}

// beginsCommand is the reserved words a command follows.
var beginsCommand = map[string]bool{"if": true, "then": true, "elif": true, "else": true, "while": true,
	"until": true, "do": true, "!": true, "{": true, "time": true}

// quotedSubscript is the `name[subscript]=` or `+=` at index, its subscript quoted, and where
// its value begins, when the subscript holds a blank or an operator and no quoting of its own.
func quotedSubscript(text string, index int) (string, int, bool) {
	open := index
	for open < len(text) && isNameByte(text[open]) {
		open++
	}
	if open >= len(text) || text[open] != '[' || !isValidVariableName(text[index:open]) {
		return "", 0, false
	}
	depth := 0
	for close := open; close < len(text); close++ {
		switch text[close] {
		case '\'', '"', '\\', '\n':
			return "", 0, false
		case '[':
			depth++
		case ']':
			if depth--; depth > 0 {
				continue
			}
			subscript, rest := text[open+1:close], text[close+1:]
			operator := "="
			if strings.HasPrefix(rest, "+=") {
				operator = "+="
			} else if !strings.HasPrefix(rest, "=") {
				return "", 0, false
			}
			if !strings.ContainsAny(subscript, " \t;&|()<>") {
				return "", 0, false
			}
			return text[index:open] + `["` + subscript + `"]` + operator, close + 1 + len(operator), true
		}
	}
	return "", 0, false
}

// shellWordEnd is where the word at index ends: at a blank, a newline or an operator that is
// not quoted, escaped or inside an expansion. It answers false for a quote that does not close.
func shellWordEnd(text string, index int) (int, bool) {
	for index < len(text) {
		switch char := text[index]; {
		case char == ' ' || char == '\t' || char == '\n' || strings.IndexByte(";&|()", char) >= 0:
			return index, true
		case char == '\\':
			index += 2
		case char == '\'':
			close := strings.IndexByte(text[index+1:], '\'')
			if close < 0 {
				return 0, false
			}
			index += close + 2
		case char == '"':
			close := doubleQuoteEnd(text, index+1)
			if close < 0 {
				return 0, false
			}
			index = close + 1
		case char == '$':
			if close := ansiQuoteClose(text, index); close >= 0 {
				index = close + 1
				continue
			}
			index = expansionEndAt(text, index)
		default:
			index++
		}
	}
	return len(text), true
}
