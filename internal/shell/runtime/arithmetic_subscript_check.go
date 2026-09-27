package runtime

import (
	"fmt"
	"strings"
)

// checkArithmeticSubscript refuses a subscript that would be expanded a second time. An
// arithmetic expression is expanded once before it is evaluated, so a `$(`, a `${...}` with
// an operator or a backquote left in a subscript came from a variable's value, and bash 5.3
// does not run it: `x='$(cmd)'; $(( a[$x] ))` is a syntax error there. Here it ran cmd, which
// made data that a script indexed an array by into code. A plain `$name` is still read, which
// `let 'b=a[$i]'` needs: its operand was quoted and has not been expanded.
//
// Only in the expression itself, depth 0. A variable's value evaluated as an expression of its
// own is expanded there, subscripts and all, in bash as here: `x='a[$(cmd)]=1'; $(( x ))` runs
// cmd in both, which Oils records as bash's bug and which this keeps, bash deciding.
func checkArithmeticSubscript(name string, depth int) error {
	reference, ok := parseArrayReference(name)
	if !ok || depth > 0 {
		return nil
	}
	text := strings.TrimSpace(reference.subscript)
	if _, plain := unwrapSubscriptParameter(text); plain || !strings.ContainsAny(text, "$`") {
		return nil
	}
	return fmt.Errorf("%s: arithmetic syntax error: operand expected (error token is %q)", text, text)
}
