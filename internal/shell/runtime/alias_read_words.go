package runtime

import "strings"

// The words and operators of text being read for aliases; see alias_read.go. A word runs to the
// first unquoted blank, newline or operator character, and takes its quotes, escapes and
// substitutions whole, so a `;` or a `)` inside them ends nothing.

// readWordEnd is where the word that begins at index ends.
func readWordEnd(text string, index int) int {
	for index < len(text) {
		char := text[index]
		switch {
		case char == '\\':
			index += 2
		case char == '\'':
			index = quoteEnd(text, index+1, '\'')
		case char == '"':
			index = readDoubleQuoteEnd(text, index+1)
		case char == '`':
			index = quoteEnd(text, index+1, '`')
		case char == '$' && index+1 < len(text) && text[index+1] == '\'':
			if end := ansiQuoteClose(text, index); end >= 0 {
				index = end + 1
			} else {
				index = len(text)
			}
		case char == '$' && index+1 < len(text) && (text[index+1] == '(' || text[index+1] == '{'):
			index = readSubstitutionEnd(text, index)
		case strings.IndexByte("?*+@!", char) >= 0 && index+1 < len(text) && text[index+1] == '(':
			// An extended pattern, `@(a|b)`, is one word, its parentheses and bars included.
			index = parenthesisEnd(text, index+1)
		case char == '=' && index+1 < len(text) && text[index+1] == '(':
			// An array literal, `a=(x y)`, is one word.
			index = parenthesisEnd(text, index+1)
		case isShellBlank(char) || strings.IndexByte("\n;&|()<>", char) >= 0:
			return index
		default:
			index++
		}
	}
	return len(text)
}

// readSubstitutionEnd is the index after the `$(...)`, `$((...))` or `${...}` that begins at index.
func readSubstitutionEnd(text string, index int) int {
	switch {
	case strings.HasPrefix(text[index:], "$(("):
		if end, ok := arithmeticExpansionEnd(text, index+3); ok {
			return end + 1
		}
	case strings.HasPrefix(text[index:], "$("):
		if end, ok := commandSubstitutionEnd(text, index+2); ok {
			return end + 1
		}
	default:
		return braceEnd(text, index+2)
	}
	return len(text)
}

// quoteEnd is the index after the quote that closes one opened before index, past an escaped one.
func quoteEnd(text string, index int, quote byte) int {
	for ; index < len(text); index++ {
		switch {
		case text[index] == '\\' && quote != '\'':
			index++
		case text[index] == quote:
			return index + 1
		}
	}
	return len(text)
}

// readDoubleQuoteEnd is the index after the `"` that closes one opened before index.
func readDoubleQuoteEnd(text string, index int) int {
	for index < len(text) {
		switch {
		case text[index] == '\\':
			index += 2
		case text[index] == '"':
			return index + 1
		case text[index] == '$' && index+1 < len(text) && (text[index+1] == '(' || text[index+1] == '{'):
			index = readSubstitutionEnd(text, index)
		case text[index] == '`':
			index = quoteEnd(text, index+1, '`')
		default:
			index++
		}
	}
	return len(text)
}

// braceEnd is the index after the `}` that closes a `${` whose body starts at index.
func braceEnd(text string, index int) int {
	for depth := 1; index < len(text); {
		switch text[index] {
		case '\\':
			index += 2
			continue
		case '\'':
			index = quoteEnd(text, index+1, '\'')
			continue
		case '"':
			index = readDoubleQuoteEnd(text, index+1)
			continue
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return index + 1
			}
		}
		index++
	}
	return len(text)
}

// parenthesisEnd is the index after the `)` that closes the `(` at index, quotes stepped over.
func parenthesisEnd(text string, index int) int {
	for depth := 0; index < len(text); {
		switch text[index] {
		case '\\':
			index += 2
			continue
		case '\'':
			index = quoteEnd(text, index+1, '\'')
			continue
		case '"':
			index = readDoubleQuoteEnd(text, index+1)
			continue
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return index + 1
			}
		}
		index++
	}
	return len(text)
}

// readOperator is the operator at index: the longest of the shell's that text holds there.
func readOperator(text string, index int) string {
	for _, operator := range [...]string{";;&", "<<<", "<<-", "&>>", ";;", ";&", "&&", "||", "|&", "<<", ">>", "<&", ">&", "<>", ">|", "&>"} {
		if strings.HasPrefix(text[index:], operator) {
			return operator
		}
	}
	return text[index : index+1]
}

// plainAliasWord is whether a word is written with no quote, escape or expansion, which an alias's
// name has to be to be read as one.
func plainAliasWord(word string) bool {
	return word != "" && strings.IndexAny(word, "'\"\\$`") < 0
}
