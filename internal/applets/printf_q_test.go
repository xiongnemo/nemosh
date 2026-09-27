package applets

import (
	"strings"
	"testing"
)

// printf %q end to end. The quoting itself is internal/shellquote's, tested there against
// bash; this is that printf reaches it, operand by operand and inside a larger format.
func TestPrintf_percentQ(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"%q", "a b"}, want: `a\ b`},
		{args: []string{"%q", ""}, want: "''"},
		{args: []string{"[%q]", "a$b"}, want: `[a\$b]`},
		{args: []string{"%q %q", "x"}, want: `x ''`},
		{args: []string{"%q\n", "one\ntwo", "a=b"}, want: "$'one\\ntwo'\na=b\n"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			var stdout, stderr strings.Builder
			applet, ok := DefaultRegistry.Lookup("printf")
			if !ok {
				t.Fatal("printf is not registered")
			}
			if err := applet.Run(t.Context(), test.args, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("printf: %v (%s)", err, stderr.String())
			}
			if stdout.String() != test.want {
				t.Fatalf("printf %q = %q, want %q", test.args, stdout.String(), test.want)
			}
		})
	}
}
