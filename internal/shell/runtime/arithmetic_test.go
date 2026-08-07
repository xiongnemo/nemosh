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

func TestRuntime_assignsInsideAnArithmeticExpansion(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "plain", script: "i=0\necho $((i=i+1))\necho [$i]\n", want: "1\n[1]\n"},
		{name: "add and assign", script: "i=5\necho $((i+=3))\necho [$i]\n", want: "8\n[8]\n"},
		{name: "subtract and assign", script: "i=5\necho $((i-=3))\n", want: "2\n"},
		{name: "multiply and assign", script: "i=5\necho $((i*=3))\n", want: "15\n"},
		{name: "divide and assign", script: "i=15\necho $((i/=3))\n", want: "5\n"},
		{name: "shift and assign", script: "i=1\necho $((i<<=4))\n", want: "16\n"},
		{name: "right associative", script: "echo $((a = b = 7))\necho [$a] [$b]\n", want: "7\n[7] [7]\n"},
		{name: "equality is not assignment", script: "x=1\necho $((x == 1))\necho [$x]\n", want: "1\n[1]\n"},
		{name: "the increment idiom in a loop", script: "i=0\nwhile test $i != 3; do : $((i+=1)); done\necho $i\n", want: "3\n"},
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

func TestRuntime_evaluatesLet(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "assigns", script: "let 'i = 2 + 3'\necho [$i]\n", want: "[5]\n"},
		{name: "non-zero is success", script: "let '1' && echo yes\n", want: "yes\n"},
		{name: "zero is failure", script: "let '0' || echo no\n", want: "no\n"},
		{name: "several expressions, last decides", script: "let 'a=1' 'b=0' || echo last-was-zero\n", want: "last-was-zero\n"},
		{name: "reads back", script: "i=1\nlet 'i += 4'\necho [$i]\n", want: "[5]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("stdout = %q, stderr = %q, want %q", stdout, stderr, test.want)
			}
		})
	}
}

func TestRuntime_reportsTheShellAndChildCpuTime(t *testing.T) {
	// Two lines, `%dm%fs` twice each, which is the shape POSIX 2.14 specifies.
	// When
	status, stdout, stderr := runSetScript(t, "times\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want two lines", stdout)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("line %q, want two times on it", line)
		}
		for _, field := range fields {
			if !strings.Contains(field, "m") || !strings.HasSuffix(field, "s") {
				t.Fatalf("time %q, want the %%dm%%fs shape", field)
			}
		}
	}
}
