package runtime

import (
	"errors"
	"fmt"
)

// let evaluates each operand as an arithmetic expression and reports whether
// the last one came out non-zero -- 0 when it did, 1 when it did not, which is
// the inversion `let` shares with `test` and which makes `if let "x > 0"` read
// the way it looks.
//
// It is not in POSIX; busybox carries it under ENABLE_FEATURE_SH_MATH
// (shell/ash.c:12099). It costs almost nothing here because `$(( ))` already
// has the evaluator, and it is the natural home for the assignment forms.
//
// An expression it cannot evaluate is status 2, busybox's. `((expr))` comes here
// as `let "expr"`, and busybox has no `((`: there it is bash's, status 1 and put
// to `((`, so that `(( a[1][2] = 3 )); echo $?` says 1 where it said 2.
func (r Runtime) let(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "let: missing expression")
		return 2
	}
	name, failed := "let", 2
	if r.arithmeticCommand {
		name, failed = "((", 1
		// Cleared, so a command the expression runs is not taken for one.
		r.arithmeticCommand = false
	}
	var last int64
	for _, expression := range args {
		value, err := r.evaluateArithmetic(expression)
		if errors.Is(err, errReadonlyTarget) {
			// Reported already, and a shell error, which the caller acts on.
			return 1
		}
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "%s: %v\n", name, err)
			return failed
		}
		last = value
	}
	if last != 0 {
		return 0
	}
	return 1
}
