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
	// subject is the word after `case`, which is the case's subject whatever it says: in `(
	// case esac in "esac") x;; esac )` the first esac closed the case, and the pattern's `)`
	// closed the subshell. Quoted and escaped characters are the word's too, so a quoted
	// subject is a word and a quoted keyword is none.
	subject := false
	var word strings.Builder
	// A word ends at a blank, a bracket, or a separator; what it was decides whether a
	// pattern begins or a case ends.
	endWord := func() {
		text := word.String()
		word.Reset()
		switch {
		case text == "":
		case subject:
			subject = false
		// Where a pattern goes, `case` is a pattern: `case case in case)`.
		case text == "case" && !pattern:
			depth++
			pattern = false
			subject = true
		case text == "in":
			if depth > 0 {
				pattern = true
			}
		case text == "esac":
			if depth > 0 {
				depth--
			}
			pattern = false
		}
	}

	quote := byte(0)
	escaped := false
	for index := 0; index < len(prefix); index++ {
		char := prefix[index]
		if end := ansiQuoteClose(prefix, index); !escaped && quote == 0 && end >= 0 {
			word.WriteString(prefix[index : end+1])
			index = end
			continue
		}
		switch {
		case escaped:
			escaped = false
			word.WriteByte(char)
		case char == '\\' && quote != '\'':
			escaped = true
			word.WriteByte(char)
		case char == '\'' && quote != '"', char == '"' && quote != '\'':
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			word.WriteByte(char)
		case quote != 0:
			// Inside quotes nothing is a keyword, so a `"case"` in a string counts for
			// nothing -- the same trade insideCase makes, for the same reason.
			word.WriteByte(char)
		case char == '$' && index+1 < len(prefix) && (prefix[index+1] == '(' || prefix[index+1] == '{'):
			// An expansion is the word's, its parentheses too: `$((i+2)))` is one pattern and
			// its `)`. Its first `)` ended the pattern, so the last was read as a subshell's.
			end, ok := expansionEnd(prefix, index)
			if !ok {
				return false
			}
			word.WriteString(prefix[index : end+1])
			index = end
		case char == '(' && index+1 < len(prefix) && prefix[index+1] == '(':
			// An arithmetic command's `;;` is its own, in `for ((i = 0;; i++))`, and a prefix
			// that ends inside one is at no pattern's place.
			end := arithmeticCommandEnd(prefix, index)
			if end == 0 {
				return false
			}
			endWord()
			index = end
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
