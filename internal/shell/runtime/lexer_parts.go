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

// markLiteralDollar notes that the `$` about to be written at offset is quoted or escaped,
// and so expands nothing.
func markLiteralDollar(marks *map[int]struct{}, offset int) {
	if *marks == nil {
		*marks = make(map[int]struct{})
	}
	(*marks)[offset] = struct{}{}
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

// quoteBoundary is the bookkeeping at a quote character. An opening one notes how long the
// word was, and a closing one that finds it no longer records the pair as an empty quoted
// part. `""` is a field of its own wherever it sits -- `$x""` with x='a ' is `a` and an empty
// field -- and with no part it left nothing to say so.
func quoteBoundary(parts []wordPart, opened, length int, quote byte, closing bool) ([]wordPart, int) {
	if !closing {
		return parts, length
	}
	if length == opened {
		context := quoteDouble
		if quote == '\'' {
			context = quoteSingle
		}
		parts = append(parts, wordPart{kind: wordPartLiteral, quote: context})
	}
	return parts, opened
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
