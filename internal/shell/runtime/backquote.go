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
	return rewriteBackquoteText(source, backquotesInScript)
}

// backquoteText is what the text being rewritten is, which decides what a `#` and a quote are.
type backquoteText byte

const (
	// backquotesInScript is a script or a line: quotes are read, and a comment is copied whole.
	backquotesInScript backquoteText = iota
	// backquotesInBody is a backquote's body, where a comment that runs to the end of it is
	// dropped: it says nothing there, and it would take the `)` the body is closed with, so
	// "`echo a #note`" was an unterminated substitution where both references say a.
	backquotesInBody
	// backquotesInDoubleQuotes is the inside of double quotes, a prompt's: nothing in it is a
	// quote or a comment, so `PS1='# '` is a prompt of `# `. Dropping a comment that ran to the
	// end everywhere emptied a root prompt's `\$`, and took every backquote after a `#` in a
	// prompt with it.
	backquotesInDoubleQuotes
)

func rewriteBackquoteText(source string, kind backquoteText) (string, error) {
	var out strings.Builder
	quote := byte(0)
	quoted := kind == backquotesInDoubleQuotes
	for index := 0; index < len(source); index++ {
		char := source[index]
		if char == '\\' && quote != '\'' && index+1 < len(source) {
			out.WriteByte(char)
			index++
			out.WriteByte(source[index])
			continue
		}
		if end := ansiQuoteClose(source, index); quote == 0 && !quoted && end >= 0 {
			out.WriteString(source[index : end+1])
			index = end
			continue
		}
		// A comment is text to its line's end, a quote or backquote in it too: `# don't` opened a
		// quote that hid every backquote after it, and "# the `iota' function" was a
		// substitution that never closed.
		if char == '#' && quote == 0 && !quoted && commentStarts(source, index) {
			end := strings.IndexByte(source[index:], '\n')
			if end < 0 && kind == backquotesInBody {
				break
			}
			if end < 0 {
				end = len(source) - index
			}
			out.WriteString(source[index : index+end])
			index += end - 1
			continue
		}
		if !quoted && (char == '\'' && quote != '"' || char == '"' && quote != '\'') {
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
		body, end, ok := backquoteBody(source, index, quote == '"' || quoted)
		if !ok {
			return "", fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
		}
		rewritten, err := rewriteBackquoteText(body, backquotesInBody)
		if err != nil {
			return "", err
		}
		// A body that begins with `(` is a subshell, and `$(` before it made `$((`, an
		// arithmetic expansion: `(cd dir && pwd)` in backquotes was an arithmetic syntax
		// error, where busybox-w32 and bash run it.
		out.WriteString("$(")
		if strings.HasPrefix(rewritten, "(") {
			out.WriteByte(' ')
		}
		out.WriteString(rewritten)
		out.WriteString(")")
		index = end
	}
	return out.String(), nil
}

// backquoteBody reads from the backquote at start to its match, dropping the
// backslashes POSIX makes special there, and reports where the match was. Inside double
// quotes a `\"` is one of them, as both references have it: "x `echo \"hi\"`" runs
// echo "hi". Its backslash was kept, and echo printed the quotes.
func backquoteBody(source string, start int, inDouble bool) (string, int, bool) {
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
		switch next := source[index+1]; {
		case next == '$' || next == '`' || next == '\\' || next == '"' && inDouble:
			body.WriteByte(next)
			index++
		default:
			body.WriteByte(char)
		}
	}
	return "", 0, false
}
