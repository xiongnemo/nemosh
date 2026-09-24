package runtime

import "strings"

// `shopt -s nocasematch`: patterns match without regard to case where a script matches a
// word rather than a file -- `case`, `[[ == != =~ ]]`, and `${x/pattern/replacement}`, which
// is exactly where bash 5.3 applies it, measured; the `#` and `%` trims are not among them
// there either. It was not an option this build had, so `shopt -s nocasematch` failed and
// the `case` after it matched as written.
//
// Folding is ASCII only, so a folded string has its original's byte offsets and a match found
// in one can be cut out of the other. Case-insensitive matching of words a script compares --
// flags, answers, extensions -- is ASCII in practice, and a fold that moved offsets would cut
// replacements in the wrong place.

func foldASCII(text string) string {
	return strings.Map(func(char rune) rune {
		if 'A' <= char && char <= 'Z' {
			return char + ('a' - 'A')
		}
		return char
	}, text)
}

func (r Runtime) noCaseMatch() bool {
	return r.options != nil && r.options.noCaseMatch
}

// matchWordPattern is a `case` arm or a `[[ == ]]` pattern against a word.
func (r Runtime) matchWordPattern(pattern, value string) bool {
	if r.noCaseMatch() {
		return matchShellPattern(foldASCII(pattern), foldASCII(value))
	}
	return matchShellPattern(pattern, value)
}

// equalWords is `[[ a == "b" ]]`, where the quoted right side is a string rather than a
// pattern -- folded too, as bash folds it.
func (r Runtime) equalWords(left, right string) bool {
	if r.noCaseMatch() {
		return foldASCII(left) == foldASCII(right)
	}
	return left == right
}
