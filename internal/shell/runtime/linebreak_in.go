package runtime

import (
	"fmt"
	"strings"
)

// joinLinebreakIn puts the `in` of a for, select or case header back on the header's line
// when it begins the next one:
//
//	for x
//	in a b; do echo "$x"; done
//
// The grammar has `for name linebreak in` and `case WORD linebreak in`, POSIX's and bash's
// both, and busybox-w32 reads them so. The passes after this one read a header from its own
// line, so the loop above ran over "$@" and never saw a or b, and the case said it had no
// `in`. Blank and comment lines between the two are gone by now.
//
// Only across a newline: the lines were cut at `;` as well, and `for i; in a b` is a syntax
// error in both references, so an `in` that starts on the header's own line stays where it is.
func joinLinebreakIn(lines []string, at []int) ([]string, []int) {
	joined := make([]string, 0, len(lines))
	joinedAt := make([]int, 0, len(at))
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if index+1 < len(lines) && startOf(at, index+1) > startOf(at, index) &&
			awaitsIn(line) && beginsWithIn(lines[index+1]) {
			line += " " + lines[index+1]
			joined, joinedAt = appendNumbered(joined, joinedAt, line, startOf(at, index))
			index++
			continue
		}
		joined, joinedAt = appendNumbered(joined, joinedAt, line, startOf(at, index))
	}
	return joined, joinedAt
}

// awaitsIn reports a line that ends in a for or select header with nothing after its name,
// or a case header with nothing after its word -- after a pipe or an and-or operator too.
func awaitsIn(line string) bool {
	if _, _, rest, ok := splitCompoundAfterPrefix(line); ok {
		line = rest
	}
	for _, keyword := range [...]string{"for", "select", "case"} {
		header, ok := compoundHeader(line, keyword)
		if !ok {
			continue
		}
		tokens, err := scanShellTokens(header)
		if err != nil || len(tokens) != 1 || tokens[0].kind != tokenWord {
			return false
		}
		return keyword == "case" || isValidVariableName(tokens[0].value)
	}
	return false
}

// beginsWithIn reports a line whose first word is `in`.
func beginsWithIn(line string) bool {
	rest, ok := strings.CutPrefix(line, "in")
	return ok && (rest == "" || isShellBlank(rest[0]))
}

// refuseBeforeDo refuses a line between a for or select header and its do, where the grammar
// has room for none: `for x`, then `echo oops`, then `do` is a syntax error in both
// references. The line was dropped, and the loop ran as though it had not been written.
func refuseBeforeDo(lines []string, span compoundSpan) error {
	if span.doIndex <= span.start+1 {
		return nil
	}
	first, _, _ := strings.Cut(lines[span.start+1], " ")
	return fmt.Errorf("syntax error: unexpected %s", first)
}
