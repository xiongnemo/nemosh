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
