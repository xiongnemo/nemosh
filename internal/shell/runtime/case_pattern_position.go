package runtime

import "strings"

// Where a case pattern goes, which is what tells its brackets from anyone else's.
//
// insideCase answers whether a `case` is open, and that is enough while the bracket in
// question could only be a `}`: a pattern's `)` cannot close a brace group. Inside a
// *subshell* it is not enough -- the innermost open bracket is a `(`, so the pattern's `)`
// looks exactly like the closer, and `( case a in a) x;; esac )` came apart at the first
// arm.
//
// What separates them is position rather than depth. A pattern follows `in` or a `;;`, and
// runs to the `)` that ends it; a subshell in an arm's *body* does not sit there. So:
//
//	( case a in a) x;; esac )      the first `)` ends a pattern, the last closes the subshell
//	case a in (a) x;; esac         the `(` opens a pattern and its `)` ends it -- neither counts
//	( case a in a) (echo x);; esac )   `(echo x)` is in a body, so it opens and closes normally
//
// The third line is why this cannot simply be "ignore brackets while a case is open", and
// the second is why it cannot be "a pattern's `)` never has a `(`": the optional leading
// parenthesis of a pattern has to cancel itself out. An earlier attempt at the second scan
// tried the depth-only rule and broke that form; the comment in syntax_separator.go records
// it.
//
// Computed from the prefix on demand, the way insideCase is, rather than threaded through
// each caller's loop as state. The callers are character scans with a dozen `continue`s
// apiece, and a flag that has to be updated on every one of those paths is a flag that will
// be missed on one of them. Lines are short.

// CasePatternPosition is casePatternPosition for the line editor, which needs the same
// answer for a different reason: a word where a pattern goes is data, and drawing `*` as a
// command that does not exist is a false statement about the line on screen.
//
// Exported rather than copied. The editor asking the grammar is the whole point -- see
// reserved_words.go, which says the same about where a command begins.
func CasePatternPosition(prefix string) bool {
	return casePatternPosition(prefix)
}

// casePatternPosition reports whether a scan that has just read prefix is sitting where a
// case pattern goes -- so that the next `(` opens no subshell and the next `)` closes none.
func casePatternPosition(prefix string) bool {
	depth := 0
	pattern := false
	var word strings.Builder
	// A word ends at a blank, a bracket, or a separator; what it was decides whether a
	// pattern begins or a case ends.
	endWord := func() {
		switch word.String() {
		case "case":
			depth++
			pattern = false
		case "in":
			if depth > 0 {
				pattern = true
			}
		case "esac":
			if depth > 0 {
				depth--
			}
			pattern = false
		}
		word.Reset()
	}

	quote := byte(0)
	escaped := false
	for index := 0; index < len(prefix); index++ {
		char := prefix[index]
		switch {
		case escaped:
			escaped = false
		case char == '\\' && quote != '\'':
			escaped = true
		case char == '\'' && quote != '"', char == '"' && quote != '\'':
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
		case quote != 0:
			// Inside quotes nothing is a keyword, so a `"case"` in a string counts for
			// nothing -- the same trade insideCase makes, for the same reason.
		case char == ';':
			endWord()
			// `;;` and `;;&` open the next pattern; a single `;` ends a command inside an
			// arm's body and leaves the position alone.
			if index+1 < len(prefix) && prefix[index+1] == ';' {
				index++
				if index+1 < len(prefix) && prefix[index+1] == '&' {
					index++
				}
				pattern = depth > 0
			}
		case char == ')':
			// The pattern ends here, whichever kind of `)` this is.
			endWord()
			pattern = false
		case char == '(', char == '|', char == '&', char == ' ', char == '\t', char == '\n':
			// A `|` separates alternatives within one pattern and a `(` may open one, so
			// neither leaves the position; they only end the word.
			endWord()
		default:
			word.WriteByte(char)
		}
	}
	endWord()
	return depth > 0 && pattern
}
