package runtime

// The word-part helpers the scanner in lexer.go builds its tokens out of. Split
// from it to stay under the 250-line ceiling when the scanner learned about
// `$'...'`; nothing else moved with them.

// escapesInsideDoubleQuotes reports whether the backslash at index starts an
// escape sequence. Outside double quotes it always does. Inside them POSIX keeps
// it special only before the characters that quoting itself is made of -- `$`,
// a backtick, a double quote, another backslash -- and before a newline;
// anywhere else the backslash is ordinary data and has to survive, which is what
// makes the quoted Windows path form in docs/design/windows-path-model.md:32
// usable. busybox-w32 ash spells the same list out in `case CBACK`
// (shell/ash.c:14518).
//
// A backslash at the end of the line counts as the newline case: continuation
// has already been joined by the time a line reaches here, so what is left is
// either a genuine trailing backslash or an unterminated quote, and both are
// reported by the caller rather than turned into data.
func escapesInsideDoubleQuotes(line string, index int, inDouble bool) bool {
	if !inDouble || index+1 >= len(line) {
		return true
	}
	switch line[index+1] {
	case '$', '`', '"', '\\':
		return true
	}
	return false
}

func quoteFor(inSingle, inDouble bool) quoteContext {
	if inSingle {
		return quoteSingle
	}
	if inDouble {
		return quoteDouble
	}
	return quoteUnquoted
}

func appendLiteralPart(parts *[]wordPart, text string, quote quoteContext) {
	if len(*parts) > 0 {
		last := &(*parts)[len(*parts)-1]
		if last.kind == wordPartLiteral && last.quote == quote {
			last.text += text
			return
		}
	}
	*parts = append(*parts, wordPart{kind: wordPartLiteral, text: text, quote: quote})
}
