package applets

import (
	"fmt"
	"regexp"
	"strings"
)

// `sed` matched a literal string. `s/[0-9]*//` replaced nothing and said nothing, because the
// substitution was a strings.Index search and every metacharacter stood for itself. Nothing
// in the support matrix or in `--help` admitted it, which made it the worst shape a gap can
// take: a script got silence where it expected work done.
//
// What both references implement is POSIX *basic* regular expressions plus the three GNU
// extensions, and BRE is not Go's syntax. The difference is almost entirely about which
// characters need a backslash, and it runs in both directions:
//
//	\(a\+\)   a group, repeated -- in Go, (a+)
//	(a+)      three literal characters and a literal plus -- in Go, \(a\+\)
//	a|b       a literal pipe; BRE has no alternation without the GNU \|
//	o\{2\}    an interval -- in Go, o{2}
//
// So a translation, not a pass-through: handing the pattern to regexp.Compile unchanged would
// make `(a+)` a group and `a|b` an alternation, which is a *different* wrong answer.
//
// Backreferences inside a pattern -- `\(a\)\1` -- are refused by name. Go's regexp is RE2,
// which has none, and matching `\1` as a literal digit would be silent and wrong.

// translateBasicRegex rewrites a POSIX BRE into Go's syntax.
func translateBasicRegex(pattern string) (string, error) {
	var out strings.Builder
	// atStart marks a position where `*` is an ordinary asterisk and `^` is an anchor:
	// the beginning of the pattern, or just inside a group or after an alternation.
	atStart := true
	for index := 0; index < len(pattern); index++ {
		char := pattern[index]
		switch char {
		case '\\':
			index++
			if index >= len(pattern) {
				return "", fmt.Errorf("trailing backslash in pattern")
			}
			text, opens, err := translateEscape(pattern[index])
			if err != nil {
				return "", err
			}
			out.WriteString(text)
			atStart = opens
		case '[':
			text, next, err := translateBracket(pattern, index)
			if err != nil {
				return "", err
			}
			out.WriteString(text)
			index = next
			atStart = false
		case '(', ')', '{', '}', '+', '?', '|':
			// Ordinary characters in BRE; the escaped forms above are the operators.
			out.WriteByte('\\')
			out.WriteByte(char)
			atStart = false
		case '.':
			// Any character, and the same spelling in both syntaxes. It needs its own
			// case because the default below quotes what it copies: without this, `.`
			// became `\.` and `s|.*world|W|` matched only `world`, so `hello world`
			// came back as `hello W` where both references answer `W`. The tests that
			// used `.` against a line containing a literal dot passed either way, which
			// is how it survived the first round.
			out.WriteByte('.')
			atStart = false
		case '*':
			// `*` with nothing to repeat is a literal asterisk, which is why `s/*/x/`
			// works on a line of stars.
			if atStart {
				out.WriteString(`\*`)
			} else {
				out.WriteByte('*')
			}
			atStart = false
		case '^':
			// An anchor only where one can start; `a^b` is three literal characters.
			if atStart {
				out.WriteByte('^')
			} else {
				out.WriteString(`\^`)
			}
		case '$':
			// An anchor only at the very end, for the same reason.
			if index == len(pattern)-1 {
				out.WriteByte('$')
			} else {
				out.WriteString(`\$`)
			}
			atStart = false
		default:
			out.WriteString(regexp.QuoteMeta(string(char)))
			atStart = false
		}
	}
	return out.String(), nil
}

// translateEscape rewrites the character after a backslash, answering whether what it emits
// leaves the scan at a position where `*` and `^` are literal again.
func translateEscape(char byte) (string, bool, error) {
	switch char {
	case '(', '|':
		// The one place a `^` may anchor and a `*` may not repeat, other than the start.
		return string(char), true, nil
	case ')', '{', '}', '+', '?':
		return string(char), false, nil
	case 'n':
		return "\n", false, nil
	case 't':
		return "\t", false, nil
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return "", false, fmt.Errorf(
			"backreference \\%c in a pattern: this build matches with RE2, which has none", char)
	default:
		// `\.`, `\*`, `\[`, `\\` and the rest: a literal, quoted for Go.
		return regexp.QuoteMeta(string(char)), false, nil
	}
}

// translateBracket copies a bracket expression, returning it and the index of its `]`.
//
// Mostly verbatim -- Go understands `[^a-z]` and `[[:digit:]]` already -- with two POSIX rules
// that Go does not share. A `]` immediately after `[` or `[^` is a literal, and a backslash
// inside a bracket is a literal rather than an escape, so it has to be doubled on the way out.
func translateBracket(pattern string, start int) (string, int, error) {
	var out strings.Builder
	out.WriteByte('[')
	index := start + 1
	if index < len(pattern) && pattern[index] == '^' {
		out.WriteByte('^')
		index++
	}
	if index < len(pattern) && pattern[index] == ']' {
		out.WriteString(`\]`)
		index++
	}
	for ; index < len(pattern); index++ {
		switch {
		case pattern[index] == ']':
			out.WriteByte(']')
			return out.String(), index, nil
		case pattern[index] == '\\':
			out.WriteString(`\\`)
		case pattern[index] == '[' && index+1 < len(pattern) && strings.IndexByte(":.=", pattern[index+1]) >= 0:
			// `[[:digit:]]` and its siblings: copied whole, because the `]` that ends
			// the class is not the one that ends the bracket.
			closer := string([]byte{pattern[index+1], ']'})
			end := strings.Index(pattern[index:], closer)
			if end < 0 {
				return "", 0, fmt.Errorf("unterminated character class in pattern")
			}
			out.WriteString(pattern[index : index+end+2])
			index += end + 1
		default:
			out.WriteByte(pattern[index])
		}
	}
	// POSIX says an unmatched `[` is a literal, and both references agree.
	return `\[`, start, nil
}

// translateReplacement rewrites a sed replacement into the form Regexp.Expand wants.
//
//	&      the whole match          -> ${0}
//	\1     the first group          -> ${1}
//	\&     a literal ampersand
//	$      a literal dollar         -> $$, because Expand would read it as a reference
func translateReplacement(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		switch char := text[index]; {
		case char == '\\' && index+1 < len(text):
			index++
			switch next := text[index]; next {
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				out.WriteString("${" + string(next) + "}")
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case '$':
				out.WriteString("$$")
			default:
				out.WriteByte(next)
			}
		case char == '&':
			out.WriteString("${0}")
		case char == '$':
			out.WriteString("$$")
		default:
			out.WriteByte(char)
		}
	}
	return out.String()
}
