package runtime

import (
	"fmt"
	"strings"
)

// An array subscript is an expression, not a literal.
//
// `${a[$i]}` gave the empty string where bash gives the element, and `${a[1+1]}`
// likewise: the subscript text went straight to strconv.Atoi, which failed, and a
// failed conversion was treated as out of range -- which is the empty string. So
// indexing an array by a loop variable, which is most of what an array is for,
// silently produced nothing.
//
// bash evaluates a subscript as arithmetic, which is why `a[i]` works without a
// dollar and `a[1+1]` works at all. This does the same, through the same evaluator,
// after stripping a `$` that the arithmetic lexer would choke on.

// resolveSubscript turns a subscript's text into an index.
//
// The `$` forms are unwrapped first because the arithmetic lexer has no `$`: it reads
// bare names, so `${a[i]}` already worked and `${a[$i]}` did not. Both are common and
// they must not disagree.
//
// What is deliberately not handled is a subscript needing full word expansion -- a
// command substitution, a nested `${...}` with an operator. Those would need the
// expansion machinery and a context this path does not carry, and they are refused
// below rather than quietly becoming zero, because zero is a valid index and would
// silently read the wrong element.
func (r Runtime) resolveSubscript(subscript string) (int, error) {
	text := strings.TrimSpace(subscript)
	if text == "" {
		return 0, fmt.Errorf("array subscript: empty")
	}
	if inner, ok := unwrapSubscriptParameter(text); ok {
		text = inner
	}
	if strings.ContainsAny(text, "$`") {
		return 0, fmt.Errorf("array subscript %q: this build evaluates a subscript as arithmetic and cannot expand one", subscript)
	}
	value, err := r.evaluateArithmetic(text)
	if err != nil {
		return 0, fmt.Errorf("array subscript %q: %w", subscript, err)
	}
	if value < 0 {
		// bash counts a negative subscript from the end. Refused here rather than
		// wrapped to zero, which would read the first element for `${a[-1]}` and
		// look like an answer.
		return 0, fmt.Errorf("array subscript %q: negative subscripts are not implemented", subscript)
	}
	return int(value), nil
}

// unwrapSubscriptParameter strips `$name` and `${name}` down to the name, so the
// arithmetic evaluator can look it up the way it looks up a bare one.
func unwrapSubscriptParameter(text string) (string, bool) {
	if inner, ok := strings.CutPrefix(text, "${"); ok {
		if name, closed := strings.CutSuffix(inner, "}"); closed && isValidVariableName(name) {
			return name, true
		}
		return "", false
	}
	if name, ok := strings.CutPrefix(text, "$"); ok && isValidVariableName(name) {
		return name, true
	}
	return "", false
}

// subscriptIndex is resolveSubscript for the callers that have nowhere to report to.
//
// An expansion runs on a word being built and there is no status to fail; the empty
// string is what a bad subscript produces, which is what bash does for one that is
// merely out of range. The distinction the error above draws still holds where a
// caller can use it -- the assignment paths report.
func (r Runtime) subscriptIndex(subscript string) (int, bool) {
	index, err := r.resolveSubscript(subscript)
	if err != nil {
		return 0, false
	}
	return index, true
}
