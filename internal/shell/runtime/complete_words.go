package runtime

import "strings"

// How a half-typed line is read for completion: which of its characters a quote holds, where
// the word at the cursor begins, and the words COMP_WORDS holds.

// completionWordStart is where readline's word ends at the end of text begins: after an open
// quote, or after the last character of breaks that no quote holds.
func completionWordStart(text, breaks string) int {
	quoted, open := completionQuoted(text)
	if open >= 0 {
		return open + 1
	}
	for index := len(text) - 1; index >= 0; index-- {
		if !quoted[index] && strings.IndexByte(breaks, text[index]) >= 0 {
			return index + 1
		}
	}
	return 0
}

// completionWords is COMP_WORDS and COMP_CWORD as bash's split_at_delims makes them: text split
// at the blanks in breaks, and each run of its other characters a word of its own, so `--a=b`
// is `--a`, `=` and `b`; and the word the cursor, at offset sentinel, is in, an empty one made
// for a cursor in the blanks between words or after the last.
func completionWords(text string, sentinel int, breaks string) ([]string, int) {
	quoted, _ := completionQuoted(text)
	isBreak := func(index int) bool { return !quoted[index] && strings.IndexByte(breaks, text[index]) >= 0 }
	isBlank := func(index int) bool { return isBreak(index) && strings.IndexByte(" \t\n", text[index]) >= 0 }
	var words []string
	cword := -1
	index := 0
	for index < len(text) && isBlank(index) {
		index++
	}
	for index < len(text) {
		start, end := index, index+1
		if isBreak(start) {
			for end < len(text) && isBreak(end) && !isBlank(end) {
				end++
			}
		} else {
			for end < len(text) && !isBreak(end) {
				end++
			}
		}
		if cword < 0 && sentinel < start {
			words, cword = append(words, ""), len(words)
		}
		words = append(words, text[start:end])
		if cword < 0 && sentinel >= start && sentinel <= end {
			cword = len(words) - 1
		}
		for index = end; index < len(text) && isBlank(index); index++ {
		}
	}
	if cword < 0 {
		if sentinel > 0 && strings.IndexByte(" \t\n", text[sentinel-1]) >= 0 || len(words) == 0 {
			words = append(words, "")
		}
		cword = len(words) - 1
	}
	return words, cword
}

// completionQuoted reports for each byte of text whether a quote or a backslash holds it -- the
// quotes and the backslash too -- or a substitution that closes, as bash's char_is_quoted and
// skip_to_delim read a half-typed line; an open `$(` or backquote holds nothing, and the cursor
// may be in its command. A quote that never closes holds the rest, and the second answer is
// where it opened, -1 when every one closes.
func completionQuoted(text string) ([]bool, int) {
	quoted := make([]bool, len(text))
	hold := func(from, to int) {
		for index := from; index <= to && index < len(text); index++ {
			quoted[index] = true
		}
	}
	for index := 0; index < len(text); index++ {
		end := -1
		switch char := text[index]; {
		case char == '\\':
			end = index + 1
		case char == '\'':
			end = strings.IndexByte(text[index+1:], '\'')
			if end < 0 {
				hold(index, len(text))
				return quoted, index
			}
			end += index + 1
		case char == '"':
			end = closingDoubleQuote(text, index+1)
			if end < 0 {
				hold(index, len(text))
				return quoted, index
			}
		case char == '$' && strings.HasPrefix(text[index:], "$("):
			if close, ok := commandSubstitutionEnd(text, index+2); ok {
				end = close
			}
		case char == '$' && strings.HasPrefix(text[index:], "${"):
			if close, ok := bracedParameterEnd(text, index+1); ok {
				end = close - 1
			}
		case char == '`':
			if close := strings.IndexByte(text[index+1:], '`'); close >= 0 {
				end = index + 1 + close
			}
		}
		if end >= index {
			hold(index, end)
			index = end
		}
	}
	return quoted, -1
}

// closingDoubleQuote is the index of the `"` that closes a double-quoted string whose text
// begins at from, a backslash escaping the character after it; -1 when none does.
func closingDoubleQuote(text string, from int) int {
	for index := from; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '"':
			return index
		}
	}
	return -1
}
