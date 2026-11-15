package runtime

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
// be missed on one of them. This said "Lines are short", and a function's body is not: a
// scan asks at every bracket, so reading the prefix from its start each time made a case in
// a function quadratic in its arms. A caller that asks along one text holds a caseScan
// (case_scan.go), which keeps what it has read and is asked the same way.

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
//
// The rules are caseScan's (case_scan.go), read here once over the whole prefix:
//
//   - the word after `case` is its subject whatever it says: in `( case esac in "esac") x;;
//     esac )` the first esac closed the case, and the pattern's `)` closed the subshell.
//     Quoted and escaped characters are the word's too, so a quoted subject is a word and a
//     quoted keyword is none;
//   - only the case's own `in`, the one after its subject, begins the patterns. A `for c in`
//     in an arm's body has one too, and taking it for the case's put the next `case` where a
//     pattern goes, and its esac closed the case it was in;
//   - where a pattern goes, `case` is a pattern: `case case in case)`;
//   - an expansion is the word's, its parentheses too: `$((i+2)))` is one pattern and its `)`;
//   - an arithmetic command's `;;` is its own, in `for ((i = 0;; i++))`, and a prefix that
//     ends inside one is at no pattern's place;
//   - `;;` and `;;&` open the next pattern, and a single `;` leaves the position alone; a `)`
//     ends the pattern, whichever kind of `)` it is; a `|` and a `(` only end a word.
func casePatternPosition(prefix string) bool {
	scan := caseScan{text: prefix}
	return scan.patternAt(len(prefix))
}
