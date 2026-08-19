package runtime

import "strings"

// Whether a `)` is a case pattern or a bracket.
//
// Three scans have to answer this, and none of them can do it from the bracket alone:
// inside `{ case a in a) x ;; esac; }` the `)` closes nothing, and inside
// `( { echo mixed; ) echo leaked; }` it is crossed delimiters. Treating every `)` as a
// pattern made the malformed input look *incomplete*, which in an interactive shell means
// sitting there asking for more of a line that can never be finished -- worse than the
// wrong message.
//
// So the question is whether a `case` is open, and that is what this answers.

// insideCase reports whether text holds a `case` that no `esac` has closed yet.
//
// Words rather than tokens. A quoted `esac` or a variable holding `case` would be
// miscounted, and that is accepted deliberately: the alternative is tokenizing a partial
// line three times per scan, and the consequence of being wrong is only which of two
// diagnostics an already-malformed line gets. A `case` that is data and a `)` that is a
// bracket have to appear together before it matters.
func insideCase(text string) bool {
	depth := 0
	for _, field := range strings.Fields(text) {
		switch strings.Trim(field, ";&|(){}") {
		case "case":
			depth++
		case "esac":
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

// An unquoted parenthesis inside `[[ ]]` is *not* handled here, and the attempt is worth
// recording. Teaching these three scans that a `(` inside `[[ ]]` belongs to the condition
// looked symmetric with the case rule above, and it broke `[[ ( a == a ) ]]` -- the nested
// condition form, which worked before -- while still not fixing `[[ x =~ (b) ]]`. The
// parentheses of a condition reach the condition parser by a route these scans are not the
// whole of, so the change has to start there rather than here.
//
// The practical consequence: a group in a regular expression only works quoted here,
// `[[ abc =~ "(b)" ]]`. That is *not* bash-compatible and the difference is worth naming
// rather than recommending -- bash 3.2 and later treat a quoted right-hand side as a
// literal string, so the same line finds nothing there. It is a pre-existing divergence
// in this shell, not something introduced for this, and it is the second half of the same
// gap: the parentheses of a condition need handling where the condition is parsed.
