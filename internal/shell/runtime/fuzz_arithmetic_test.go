package runtime

import (
	"io"
	"testing"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Arithmetic takes text a script computed -- `$(( $x ))` evaluates whatever x holds -- so
// an expression that panics the evaluator takes the shell down with it. Division and
// remainder by zero, the most negative number divided by -1, shifts past the width, a base
// that is no base, a subscript that nests: each is an error or a number, never a panic.
func FuzzEvaluateArithmetic(f *testing.F) {
	for _, seed := range []string{
		"1+2", "x=5, x*2", "a[1]=3, a[1]**2", "2**63", "-9223372036854775808 / -1",
		"-9223372036854775808 % -1", "10 % 0", "10 / 0", "1 ? 2 : 3", "((1))", "x++ + ++x",
		"1 << 64", "1 >> -1", "08", "0x", "64#@", "2#", "37#1", "~0", "!x", "a[a[0]]",
		"y = (x = 2) * 3", "x ^= 1, x |= 2, x &= 3", "2 ** -1", "--x", "x--- -y",
	} {
		f.Add(seed)
	}
	// One shell for every input, so the variables one input assigns are there for the next to
	// read, nest and index, which reaches further into the evaluator than a fresh shell each.
	r := New(applets.DefaultRegistry, Streams{Stdout: io.Discard, Stderr: io.Discard})
	f.Fuzz(func(t *testing.T, expression string) {
		if !utf8.ValidString(expression) || len(expression) > 512 {
			t.Skip()
		}
		_, _ = r.evaluateArithmetic(expression)
	})
}
