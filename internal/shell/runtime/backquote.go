package runtime

import (
	"fmt"
	"strings"
)

// rewriteBackquotes turns `command` into $(command) before anything else reads
// the source. POSIX 2.6.3 gives the two forms the same meaning and differs only
// in how a backslash inside them is read, so translating at the front means the
// line scanner, the separator splitter, the group extractor, and the lexer each
// have one form to know about instead of two. Nemosh recognised neither of
// them in backquote spelling: `echo hi` reached the command line as its own
// literal text, backquotes and all.
//
// Inside `...` a backslash is special only before `$`, another backquote, and
// itself, so those pairs lose the backslash on the way into the $( ) body and
// every other backslash survives untouched. An escaped backquote is what nests
// one substitution inside another, and unescaping it here is exactly what turns
// the inner pair back into an ordinary one for the recursive pass.
func rewriteBackquotes(source string) (string, error) {
	var out strings.Builder
	quote := byte(0)
	for index := 0; index < len(source); index++ {
		char := source[index]
		if char == '\\' && quote != '\'' && index+1 < len(source) {
			out.WriteByte(char)
			index++
			out.WriteByte(source[index])
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			out.WriteByte(char)
			continue
		}
		if quote == '\'' || char != '`' {
			out.WriteByte(char)
			continue
		}
		body, end, ok := backquoteBody(source, index)
		if !ok {
			return "", fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
		}
		rewritten, err := rewriteBackquotes(body)
		if err != nil {
			return "", err
		}
		out.WriteString("$(")
		out.WriteString(rewritten)
		out.WriteString(")")
		index = end
	}
	return out.String(), nil
}

// backquoteBody reads from the backquote at start to its match, dropping the
// backslashes POSIX makes special there, and reports where the match was.
func backquoteBody(source string, start int) (string, int, bool) {
	var body strings.Builder
	for index := start + 1; index < len(source); index++ {
		char := source[index]
		if char == '`' {
			return body.String(), index, true
		}
		if char != '\\' || index+1 >= len(source) {
			body.WriteByte(char)
			continue
		}
		switch source[index+1] {
		case '$', '`', '\\':
			body.WriteByte(source[index+1])
			index++
		default:
			body.WriteByte(char)
		}
	}
	return "", 0, false
}
