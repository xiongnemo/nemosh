package main

import "strings"

// Completing inside a quote the user opened: `ls "My Do<TAB>`.
//
// The word was cut at the last blank, quoted or not, so `"My Do` was completed as a word
// called `Do`, and a name went back with backslashes before its blanks, which inside a quote
// are literal. Here the word is read as the shell reads it, and a name goes back quoted the
// way the word was.

// typedWordValue is what a typed word means once its quoting is taken off, and the quote left
// open at its end, if any. Outside quotes a backslash takes the next character literally;
// inside single quotes nothing is special; inside double quotes a backslash escapes only `"`,
// `\`, `$` and a backquote, as the shell reads them there, and stands for itself otherwise.
func typedWordValue(typed string) (value string, open rune) {
	var plain strings.Builder
	runes := []rune(typed)
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		switch {
		case open == '\'':
			if r == '\'' {
				open = 0
			} else {
				plain.WriteRune(r)
			}
		case r == '\\' && index+1 < len(runes):
			next := runes[index+1]
			if open == '"' && !strings.ContainsRune("\"\\$`", next) {
				plain.WriteRune(r)
				continue
			}
			plain.WriteRune(next)
			index++
		case open == '"':
			if r == '"' {
				open = 0
			} else {
				plain.WriteRune(r)
			}
		case r == '"' || r == '\'':
			open = r
		default:
			plain.WriteRune(r)
		}
	}
	return plain.String(), open
}

// quoteForInsertion writes a completed name inside the quote the word was opened with.
//
// The whole word is written again rather than only the part Tab added, because the spelling
// can change -- on Windows the case comes from the name on disk -- and quoting it whole names
// the same file. A leading `~/` stays outside the quote, where it is still expanded.
func quoteForInsertion(text string, quote rune) string {
	prefix := ""
	if rest, found := strings.CutPrefix(text, "~/"); found {
		prefix, text = "~/", rest
	}
	if quote == '\'' {
		return prefix + "'" + strings.ReplaceAll(text, "'", `'\''`)
	}
	var quoted strings.Builder
	quoted.WriteString(prefix + `"`)
	for _, r := range text {
		if strings.ContainsRune("\"\\$`", r) {
			quoted.WriteByte('\\')
		}
		quoted.WriteRune(r)
	}
	return quoted.String()
}
