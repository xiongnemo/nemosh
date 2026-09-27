package runtime

import "strings"

func parseTypedWord(result word) word {
	if len(result.parts) == 0 {
		result.parts = []wordPart{{kind: wordPartLiteral}}
	}
	return result
}

func parameterEnd(text string, start int) int {
	if start < len(text) && text[start] == '{' {
		if end, ok := bracedParameterEnd(text, start); ok {
			return end
		}
	}
	if start < len(text) && strings.ContainsRune("@*#?$!-", rune(text[start])) {
		return start + 1
	}
	if start < len(text) && text[start] >= '0' && text[start] <= '9' {
		return start + 1
	}
	end := start
	for end < len(text) && isNameByte(text[end]) {
		end++
	}
	if end == start {
		return start
	}
	return end
}

func containsError(err, target error) bool {
	return strings.Contains(err.Error(), target.Error())
}

// bracedParameterEnd finds the `}` that closes the `${` at start, counting nesting.
//
// It used to take the *first* `}` in the rest of the text, which is wrong the moment
// one expansion appears inside another -- and `${VAR:-${DEFAULT}}` is one of the
// commonest lines in any shell script. Measured before this: it printed `}`, because
// the reference was cut at `${x:-${y` and the trailing brace became literal text. A
// nested default silently produced a stray brace instead of a value.
//
// Quotes are skipped, so a `}` written as data inside the expansion -- `${x:-"}"}` --
// does not end it either. Only a `${` nests: a bare `{` is text, and the first `}` after it
// ends the expansion, in busybox-w32 and bash alike -- `${x:-{b}}` with x set is a and a }.
// Every `{` was counted, so that ran on to the second `}` and the } after it was lost.
func bracedParameterEnd(text string, start int) (int, bool) {
	depth := 0
	quote := byte(0)
	dollar := -1
	for index := start; index < len(text); index++ {
		char := text[index]
		// A backslash escapes the next character everywhere but inside single quotes, so
		// `${x:-a\"b}` is one expansion: the escaped quote opened a quote that never closed,
		// and the whole reference came out as its own text.
		if char == '\\' && quote != '\'' {
			index++
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else if end, ok := substitutionEnd(text, index); ok && quote == '"' {
				// Inside double quotes a substitution is still one, quotes of its own and all.
				index = end
			}
			continue
		}
		if end := ansiQuoteClose(text, index); end >= 0 {
			index = end
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '$':
			dollar = index
			// A command substitution or an arithmetic expansion is stepped over whole, so a
			// `}` in one does not end this: `${x:-$({ echo hi; })}`, and a backquoted `echo
			// }`, which is one by now. The group's `}` ended it and left `)}` behind.
			if end, ok := substitutionEnd(text, index); ok {
				index = end
			}
		case '{':
			if index == start || dollar == index-1 {
				depth++
			}
		case '}':
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return 0, false
}

// substitutionEnd is the last byte of the `$(...)` or `$((...))` at index, and false when
// there is none there or it does not close.
func substitutionEnd(text string, index int) (int, bool) {
	if index+1 >= len(text) || text[index] != '$' || text[index+1] != '(' {
		return 0, false
	}
	if index+2 < len(text) && text[index+2] == '(' {
		if end, ok := arithmeticExpansionEnd(text, index+3); ok {
			return end, true
		}
	}
	return commandSubstitutionEnd(text, index+2)
}
