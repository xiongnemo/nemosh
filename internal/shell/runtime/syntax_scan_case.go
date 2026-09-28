package runtime

import "strings"

// casePattern is whether the scan sits where a case pattern goes, so that a `(` or `)` there
// is the pattern's and no group's. In `( case a in` with `a) x;;` on the next line, the
// pattern's `)` closed the subshell, and the line was cut in the middle of the case: "unexpected
// esac". Both references run it. The same held for a subshell in a brace group, which ended
// in "unexpected ), expected }".
func (scanner *syntaxScanner) casePattern() bool {
	text := scanner.logical.String()
	return strings.Contains(text, "case") && casePatternPosition(text)
}
