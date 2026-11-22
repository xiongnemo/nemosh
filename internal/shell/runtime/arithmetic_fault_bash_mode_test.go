package runtime

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode an arithmetic error is said in bash's words: the expression with its leading
// blanks gone, bash's message, and the expression from the token it was found at to its end;
// let and (( put their names in front, an arithmetic for loop's is ((, and an error inside a
// variable's value or a subscript names that expression alone. busybox's words, which nemosh and
// sh keep, are otherwise. Each line measured 2026-11-22 with Git for Windows' bash 5.3.
func TestArithmeticFault_bashModeSaysItInBashsWords(t *testing.T) {
	cases := []struct{ code, said string }{
		{"echo $(( 1 2 ))", `1 2 : arithmetic syntax error in expression (error token is "2 ")`},
		{"echo $(( 3 = 4 ))", `3 = 4 : attempted assignment to non-variable (error token is "= 4 ")`},
		{"echo $(( 1 ? 2 ))", "1 ? 2 : `:' expected for conditional expression (error token is \"2 \")"},
		{"echo $(( 1 ? 2 3 ))", "1 ? 2 3 : `:' expected for conditional expression (error token is \"3 \")"},
		{"echo $(( 1 @ 2 ))", `1 @ 2 : arithmetic syntax error: invalid arithmetic operator (error token is "@ 2 ")`},
		{"echo $(( 1.5 ))", `1.5 : arithmetic syntax error: invalid arithmetic operator (error token is ".5 ")`},
		{"echo $(( 1 + ))", `1 + : arithmetic syntax error: operand expected (error token is "+ ")`},
		{"echo $(( 08 + 1 ))", `08: value too great for base (error token is "08")`},
		{"echo $(( 2#3 ))", `2#3: value too great for base (error token is "2#3")`},
		{"echo $(( 99#1 ))", `99#1: invalid arithmetic base (error token is "99#1")`},
		{"echo $(( 10# ))", `10#: invalid integer constant (error token is "10#")`},
		{"echo $(( 1/0 ))", `1/0 : division by 0 (error token is "0 ")`},
		{"echo $(( 1/0 + 2 ))", `1/0 + 2 : division by 0 (error token is "0 + 2 ")`},
		{"echo $(( 2 ** -1 ))", `2 ** -1 : exponent less than 0 (error token is "1 ")`},
		{"x='x+1'; echo $(( x ))", `x+1: expression recursion level exceeded (error token is "x+1")`},
		{"y=1/0; echo $(( y + 1 ))", `1/0: division by 0 (error token is "0")`},
		{"a=(1 2); echo ${a[1/0]}", `1/0: division by 0 (error token is "0")`},
		{"for (( i = 0; i < 1/0; i++ )); do :; done", `((: i < 1/0: division by 0 (error token is "0")`},
		{"(( 1/0 ))", `((: 1/0 : division by 0 (error token is "0 ")`},
		{"let 1/0", `let: 1/0: division by 0 (error token is "0")`},
		{"let '(1+2'", "let: (1+2: missing `)' (error token is \"2\")"},
		{"let 'x='", `let: x=: arithmetic syntax error: operand expected (error token is "=")`},
		{"let ')'", `let: ): arithmetic syntax error: operand expected (error token is ")")`},
		{"let 'a[1'", `let: a[1: bad array subscript (error token is "a[1")`},
		{"declare -i n; n='2 3'", `2 3: arithmetic syntax error in expression (error token is "3")`},
	}
	var script, want strings.Builder
	for index, test := range cases {
		fmt.Fprintf(&script, "%s\n", test.code)
		fmt.Fprintf(&want, "nemosh: line %d: %s\n", index+1, test.said)
	}
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.EnterBashMode()
	rt.RunScript(context.Background(), script.String())
	got, expected := strings.Split(stderr.String(), "\n"), strings.Split(want.String(), "\n")
	for index := range max(len(got), len(expected)) {
		if index >= len(got) || index >= len(expected) || got[index] != expected[index] {
			t.Errorf("stderr line %d:\n got  %q\n want %q", index+1, at(got, index), at(expected, index))
		}
	}
	// nemosh and sh keep their words.
	stderr.Reset()
	New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}).RunScript(context.Background(), "let 1/0\nlet '1 2'\n")
	if want := "nemosh: line 1: let: divide by zero\nnemosh: line 2: let: arithmetic syntax error: unexpected \"2\"\n"; stderr.String() != want {
		t.Errorf("nemosh's words: %q, want %q", stderr.String(), want)
	}
}

func at(lines []string, index int) string {
	if index < len(lines) {
		return lines[index]
	}
	return "(none)"
}
