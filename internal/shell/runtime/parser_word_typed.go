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
// does not end it either.
func bracedParameterEnd(text string, start int) (int, bool) {
	depth := 0
	quote := byte(0)
	for index := start; index < len(text); index++ {
		char := text[index]
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
	}
	return 0, false
}
