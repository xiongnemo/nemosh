package runtime_test

import (
	"strings"
	"testing"
)

func TestRuntime_evaluatesArithmeticExpansion(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "addition", script: "echo $((1+2))\n", want: "3\n"},
		{name: "precedence", script: "echo $((2+3*4))\n", want: "14\n"},
		{name: "parentheses", script: "echo $(((2+3)*4))\n", want: "20\n"},
		{name: "nested parentheses", script: "echo $(( (1+2) * (3+4) ))\n", want: "21\n"},
		{name: "subtraction and unary minus", script: "echo $((1 - -2))\n", want: "3\n"},
		{name: "division truncates", script: "echo $((7/2))\n", want: "3\n"},
		{name: "modulo", script: "echo $((7%3))\n", want: "1\n"},
		{name: "shifts", script: "echo $((1<<4)) $((32>>2))\n", want: "16 8\n"},
		{name: "comparison", script: "echo $((3<4)) $((4<3))\n", want: "1 0\n"},
		{name: "equality", script: "echo $((3==3)) $((3!=3))\n", want: "1 0\n"},
		{name: "logical and or", script: "echo $((1&&0)) $((1||0))\n", want: "0 1\n"},
		{name: "logical not", script: "echo $((!0)) $((!5))\n", want: "1 0\n"},
		{name: "bitwise", script: "echo $((6&3)) $((6|3)) $((6^3)) $((~0))\n", want: "2 7 5 -1\n"},
		{name: "ternary", script: "echo $((1?10:20)) $((0?10:20))\n", want: "10 20\n"},
		{name: "a variable by name", script: "n=7\necho $((n*3))\n", want: "21\n"},
		{name: "an unset name is zero", script: "echo $((nosuch+5))\n", want: "5\n"},
		{name: "hexadecimal", script: "echo $((0x10))\n", want: "16\n"},
		{name: "the increment idiom", script: "i=0\ni=$((i+1))\ni=$((i+1))\necho $i\n", want: "2\n"},
		{name: "inside a loop condition", script: "i=0\nwhile test $i != 3; do i=$((i+1)); done\necho $i\n", want: "3\n"},
		{name: "quoted still expands", script: "echo \"[$((2*3))]\"\n", want: "[6]\n"},
		{name: "single quotes do not", script: "echo '$((2*3))'\n", want: "$((2*3))\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func TestRuntime_reportsDivisionByZero(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo $((1/0))\necho STILL\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
	if !strings.Contains(stderr, "division by zero") {
		t.Fatalf("stderr = %q, want a division-by-zero diagnostic", stderr)
	}
}

func TestRuntime_reportsAnArithmeticSyntaxError(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "echo $((1 + ))\n")

	// Then
	if status != 2 || !strings.Contains(stderr, "arithmetic") {
		t.Fatalf("status = %d, stderr = %q, want 2 and an arithmetic diagnostic", status, stderr)
	}
}

func TestRuntime_saysAssignmentIsUnsupported_whenAnArithmeticExpansionUsesIt(t *testing.T) {
	// `i=$((i+1))` is the form that needs no in-expansion assignment, and it is
	// the one this shell promises. Saying so beats mis-parsing it.
	// When
	status, _, stderr := runSetScript(t, "i=0\necho $((i=i+1))\n")

	// Then
	if status != 2 || !strings.Contains(stderr, "assignment") {
		t.Fatalf("status = %d, stderr = %q, want 2 and an assignment diagnostic", status, stderr)
	}
}
