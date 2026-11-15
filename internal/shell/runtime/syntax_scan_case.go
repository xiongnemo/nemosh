package runtime

// casePattern is whether the scan sits where a case pattern goes, so that a `(` or `)` there
// is the pattern's and no group's. In `( case a in` with `a) x;;` on the next line, the
// pattern's `)` closed the subshell, and the line was cut in the middle of the case: "unexpected
// esac". Both references run it. The same held for a subshell in a brace group, which ended
// in "unexpected ), expected }".
//
// Asked of the logical line so far, which a function's body makes the whole function: it was
// read from its start at every bracket, after a strings.Contains of it for "case", so a body
// was read once per bracket in it. The scan keeps what it has read of the line.
func (scanner *syntaxScanner) casePattern() bool {
	text := scanner.logical.String()
	scanner.cases.extend(text)
	return scanner.cases.patternAt(len(text))
}

// caseOpen is insideCase of the logical line so far, kept the same way.
func (scanner *syntaxScanner) caseOpen() bool {
	text := scanner.logical.String()
	scanner.words.extend(text)
	return scanner.words.openAt(len(text))
}
