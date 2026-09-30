package main

import "strings"

// The words history expansion takes a line apart into, and the quoting it reads and writes,
// as bash's (lib/readline/histexpand.c, and bashhist.c for what bash sets).

const (
	// historyNoExpandChars may follow a `!` that is then no expansion: bash's own list,
	// bash_history_no_expand_chars, which adds the operators to readline's.
	historyNoExpandChars = " \t\n\r=;&|()<>"
	// historyWordDelimiters end a word of a line, and are words themselves.
	historyWordDelimiters = " \t\n;&()|<>"
	// historySearchDelimiters end a `!string` search.
	historySearchDelimiters = ";&()|<>"
	// historyEventDelimiters end a `!string` search too, a `-` only after its first
	// character: they begin a word designator.
	historyEventDelimiters = "^$*%-"
)

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHistoryBlank(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

func byteIn(set string, c byte) bool { return c != 0 && strings.IndexByte(set, c) >= 0 }

// historyTokenize is history_tokenize: the line split where the shell would split it, quotes
// kept in their words and each operator a word of its own, so `echo "a b"|wc` is echo, "a b",
// | and wc. A `#` that begins a word ends the words.
func historyTokenize(line string) []string {
	words, _ := historyTokenizeAt(line, -1)
	return words
}

// historyTokenizeAt is history_tokenize_internal: the words, and which of them holds the
// byte at index, or -1.
func historyTokenizeAt(line string, index int) ([]string, int) {
	var words []string
	holding := -1
	for i := 0; i < len(line); {
		for i < len(line) && isHistoryBlank(line[i]) {
			i++
		}
		if i >= len(line) || line[i] == '#' {
			break
		}
		start := i
		i = historyWordEnd(line, start)
		if i == start {
			// A delimiter no blank skip took, and those after it, are one word.
			for i++; i < len(line) && byteIn(historyWordDelimiters, line[i]); i++ {
			}
		}
		if index >= start && index < i {
			holding = len(words)
		}
		words = append(words, line[start:i])
	}
	return words, holding
}

// historyWordEnd is history_tokenize_word: where the word at start ends.
func historyWordEnd(s string, start int) int {
	at := func(k int) byte {
		if k < len(s) {
			return s[k]
		}
		return 0
	}
	i := start
	var delimiter, open byte
	nesting := 0
	if byteIn("()\n", at(i)) {
		return i + 1
	}
	if isDigit(at(i)) {
		j := i
		for isDigit(at(j)) {
			j++
		}
		if j >= len(s) {
			return j
		}
		i = j
		if at(j) != '<' && at(j) != '>' {
			return historyQuotedWordEnd(s, i, 0, 0, 0)
		}
	}
	if byteIn("<>;&|", at(i)) {
		peek := at(i + 1)
		switch {
		case peek == at(i):
			if peek == '<' && (at(i+2) == '-' || at(i+2) == '<') {
				i++
			}
			return i + 2
		case peek == '&' && (at(i) == '>' || at(i) == '<'):
			j := i + 2
			for isDigit(at(j)) {
				j++
			}
			if at(j) == '-' {
				j++
			}
			return j
		case peek == '>' && at(i) == '&' || peek == '|' && at(i) == '>':
			return i + 2
		case peek == '(' && (at(i) == '>' || at(i) == '<'):
			delimiter, open, nesting = ')', '(', 1
			return historyQuotedWordEnd(s, i+2, delimiter, open, nesting)
		}
		return i + 1
	}
	return historyQuotedWordEnd(s, i, delimiter, open, nesting)
}

// historyQuotedWordEnd is history_tokenize_word's get_word: a word's end past its quotes, its
// escapes and its $(...), <(...) and extended-glob groups.
func historyQuotedWordEnd(s string, i int, delimiter, open byte, nesting int) int {
	at := func(k int) byte {
		if k < len(s) {
			return s[k]
		}
		return 0
	}
	if delimiter == 0 && byteIn("\"'`", at(i)) {
		delimiter = at(i)
		i++
	}
	for ; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && at(i+1) == '\n':
			i++
			continue
		case c == '\\' && delimiter != '\'' && (delimiter != '"' || byteIn("\\`\"$", c)):
			i++
			if i >= len(s) {
				return i
			}
			continue
		case nesting > 0 && c == open:
			nesting++
			continue
		case nesting > 0 && c == delimiter:
			if nesting--; nesting == 0 {
				delimiter = 0
			}
			continue
		case delimiter != 0 && c == delimiter:
			delimiter = 0
			continue
		case nesting == 0 && delimiter == 0 && byteIn("<>$!@?+*", c) && at(i+1) == '(':
			i++
			if i+1 >= len(s) {
				return i
			}
			delimiter, open, nesting = ')', '(', 1
			continue
		case delimiter == 0 && byteIn(historyWordDelimiters, c):
			return i
		case delimiter == 0 && byteIn("\"'`", c):
			delimiter = c
		}
	}
	return i
}

// historyFindWord is history_find_word: the word of line that holds the byte at index, which
// `%` is after a `!?string?` search; empty when a blank holds it.
func historyFindWord(line string, index int) string {
	words, holding := historyTokenizeAt(line, index)
	if holding < 0 {
		return ""
	}
	return words[holding]
}

// singleQuoteEnd is hist_string_extract_single_quoted: the index of the quote that closes
// the one before start, or the line's end. In $'...' a backslash escapes it.
func singleQuoteEnd(s string, start int, dollar bool) int {
	i := start
	for ; i < len(s) && s[i] != '\''; i++ {
		if dollar && s[i] == '\\' && i+1 < len(s) {
			i++
		}
	}
	return i
}

// shSingleQuote is sh_single_quote, :q's quoting.
func shSingleQuote(s string) string {
	if s == "'" {
		return `\'`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// quoteBreaks is quote_breaks, :x's: :q's quoting, broken at each blank and newline.
func quoteBreaks(s string) string {
	var out strings.Builder
	out.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\'':
			out.WriteString(`'\''`)
		case isHistoryBlank(c):
			out.WriteByte('\'')
			out.WriteByte(c)
			out.WriteByte('\'')
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte('\'')
	return out.String()
}
