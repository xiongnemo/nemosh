package runtime

import "fmt"

// let evaluates each operand as an arithmetic expression and reports whether
// the last one came out non-zero -- 0 when it did, 1 when it did not, which is
// the inversion `let` shares with `test` and which makes `if let "x > 0"` read
// the way it looks.
//
// It is not in POSIX; busybox carries it under ENABLE_FEATURE_SH_MATH
// (shell/ash.c:12099). It costs almost nothing here because `$(( ))` already
// has the evaluator, and it is the natural home for the assignment forms.
func (r Runtime) let(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "let: missing expression")
		return 2
	}
	var last int64
	for _, expression := range args {
		value, err := r.evaluateArithmetic(expression)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "let: %v\n", err)
			return 2
		}
		last = value
	}
	if last != 0 {
		return 0
	}
	return 1
}
