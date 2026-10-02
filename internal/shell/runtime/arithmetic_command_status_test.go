package runtime_test

import (
	"strings"
	"testing"
)

// An expression `((expr))` cannot evaluate is status 1 and is put to `((`, as bash has it --
// busybox has no `((` -- where `let` keeps busybox's status 2. `((` was let's 2, and said
// `let:`, a command the script never ran.
//
// Every expectation is measured against bash 5.3, and let's against busybox-w32.
func TestArithmeticCommand_failsAsBashsDoes(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		want     string
		fragment string
	}{
		{name: "an expression ended early", script: "(( 1+ )); echo s=$?\n", want: "s=1\n", fragment: "((: arithmetic syntax error"},
		{name: "two subscripts", script: "(( a[1][2] = 3 )); echo s=$?\n", want: "s=1\n", fragment: "((: "},
		{name: "division by zero", script: "(( 1/0 )); echo s=$?\n", want: "s=1\n", fragment: "((: "},
		{name: "as a condition", script: "if (( 1+ )); then echo yes; else echo no; fi\n", want: "no\n", fragment: "((: "},
		{name: "let keeps busybox's status", script: "let '1+'; echo s=$?\n", want: "s=2\n", fragment: "let: arithmetic syntax error"},
		{name: "a let inside a (( )) loop", script: "while (( 1 )); do let '1+'; echo s=$?; break; done\n", want: "s=2\n", fragment: "let: "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
			if !strings.Contains(stderr, test.fragment) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.fragment)
			}
		})
	}
}
