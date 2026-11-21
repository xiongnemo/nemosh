package runtime

import (
	"errors"
	"strings"
)

// quotedParameterEnd is parameterEnd for the lexer, which says whether the `$` before start is
// inside double quotes, where an apostrophe may close nothing; see apostropheParameterEnd.
func quotedParameterEnd(text string, start int, inDouble bool) (int, error) {
	end := parameterEnd(text, start)
	if end == start && inDouble && start < len(text) && text[start] == '{' {
		return apostropheParameterEnd(text, start)
	}
	return end, nil
}

// apostropheParameterEnd is where a double-quoted `${` at start ends when bracedParameterEnd
// found no end, and the error when it has none. A single quote that closes nothing is why, when
// the expansion ends with apostrophes read as characters: after -, + and = (and ?), with or
// without the colon, it is one, as busybox reads it, and bash before 5.2 -- `"${X-it's}"` is
// it's; after a pattern operator it opens a quote in both references, which the line does not
// close: `"${foo%d'}"` is a syntax error. Either was printed as written, which neither does.
// bash 5.3 reads a quote after - as one too, and refuses `"${X-it's}"`; both names read it as
// busybox does. Anything else still finds no end, and the `$` is a character, as before.
func apostropheParameterEnd(text string, start int) (int, error) {
	end, ok := bracedParameterEndQuoting(text, start, false)
	if !ok {
		return start, nil
	}
	if !defaultingOperator(text, start+1) {
		return 0, errors.New("unterminated quote")
	}
	return end, nil
}

// defaultingOperator reports whether the `${` whose name begins at index has an operator whose
// word is a default, an alternative or a message: -, +, = or ?, after a colon or not.
func defaultingOperator(text string, index int) bool {
	if index < len(text) && text[index] == '!' {
		index++
	}
	switch {
	case index < len(text) && isNameByte(text[index]):
		for index < len(text) && isNameByte(text[index]) {
			index++
		}
	case index < len(text) && strings.IndexByte("@*#?$!-", text[index]) >= 0:
		index++
	}
	if index < len(text) && text[index] == '[' {
		if close := strings.IndexByte(text[index:], ']'); close >= 0 {
			index += close + 1
		}
	}
	if index < len(text) && text[index] == ':' {
		index++
	}
	return index < len(text) && strings.IndexByte("-+=?", text[index]) >= 0
}
