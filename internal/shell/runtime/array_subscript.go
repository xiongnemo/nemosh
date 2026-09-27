package runtime

import (
	"context"
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
func (r Runtime) resolveSubscript(ctx context.Context, subscript string) (int, error) {
	text := strings.TrimSpace(withoutDoubleQuotes(subscript))
	if inner, ok := unwrapSubscriptParameter(text); ok {
		text = inner
	} else if strings.ContainsAny(text, "$`") {
		// Anything else with a dollar in it -- `a[$(cmd)]`, `a[${#x}]` -- is expanded
		// before it is evaluated, the same way arithmetic is. This used to be refused
		// outright, which was better than reading the wrong element but is not an
		// answer.
		text = strings.TrimSpace(r.expandEmbeddedParameters(ctx, text, 0))
	}
	// A blank subscript is 0, as bash 5.3's empty expression is: `a[" "]=x` is element 0
	// there, where it was "array subscript: empty".
	if text == "" {
		return 0, nil
	}
	value, err := r.evaluateArithmetic(text)
	if err != nil {
		// An arithmetic error, which ends the script as one in `$((...))` does, bash's answer
		// with busybox having no arrays; see reportExpansionError. It was said and passed
		// over, so `a[1+]=2` went on with status 0 and `${a[1+]}` was empty. The callers
		// stop at it without saying it again.
		err = fmt.Errorf("array subscript %q: %w", subscript, err)
		r.reportExpansionError(err)
		return 0, err
	}
	// A negative subscript is returned as it is and counted from the end by the
	// caller, which is the only place that knows how long the array is. bash does the
	// same, and refuses one that reaches past the start rather than clamping it:
	// `${a[-9]}` on three elements is `bad array subscript`, not the first element.
	return int(value), nil
}

// withoutDoubleQuotes is an indexed subscript with its double quotes removed, which bash
// removes before the arithmetic: `${a["0"]}` and `${a["$i"]}` are elements, where they were
// arithmetic errors that read as nothing. Not a single quote, which bash leaves to be the
// error, nor a quote escaped or inside a command substitution, nor one without its pair:
// `a[1"]` is bash's bad substitution, and stays an error.
func withoutDoubleQuotes(subscript string) string {
	if !strings.Contains(subscript, `"`) {
		return subscript
	}
	var out strings.Builder
	depth, quotes := 0, 0
	for index := 0; index < len(subscript); index++ {
		switch char := subscript[index]; {
		case char == '\\' && index+1 < len(subscript):
			out.WriteString(subscript[index : index+2])
			index++
			continue
		case char == '(' && index > 0 && subscript[index-1] == '$', char == '(' && depth > 0:
			depth++
		case char == ')' && depth > 0:
			depth--
		case char == '"' && depth == 0:
			quotes++
			continue
		}
		out.WriteByte(subscript[index])
	}
	if quotes%2 != 0 {
		return subscript
	}
	return out.String()
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
func (r Runtime) subscriptIndex(ctx context.Context, subscript string, length int) (int, bool) {
	index, err := r.resolveSubscript(ctx, subscript)
	if err != nil {
		return 0, false
	}
	return countFromEnd(index, length)
}

// countFromEnd turns a negative subscript into a real one. `${a[-1]}` is the last
// element; one that reaches past the start is not an element at all.
func countFromEnd(index, length int) (int, bool) {
	if index >= 0 {
		return index, true
	}
	index += length
	if index < 0 {
		return 0, false
	}
	return index, true
}
