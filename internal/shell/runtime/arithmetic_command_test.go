package runtime_test

import (
	"strings"
	"testing"
)

// `((expr))` parsed as a subshell containing a subshell, so `((i++))` ran a command
// named `i++` and reported it not found. It is how nearly every counted loop is
// written.
//
// bash documents it as equivalent to `let "expr"`, and this shell already had `let`,
// so that is what it becomes -- nothing new evaluates arithmetic, because a second
// evaluator would be a second set of answers.
func TestArithmeticCommand_evaluatesAndSetsTheStatus(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The status is the inverse of the value, which is what makes this
			// usable as a condition.
			name: "a non-zero value succeeds", script: "((1)); echo $?\n", want: "0\n",
		},
		{name: "a zero value fails", script: "((0)); echo $?\n", want: "1\n"},
		{name: "an assignment", script: "((i=5)); echo $i\n", want: "5\n"},
		{name: "an increment", script: "i=0\n((i++))\necho $i\n", want: "1\n"},
		{name: "twice", script: "i=0\n((i++))\n((i++))\necho $i\n", want: "2\n"},
		{name: "arithmetic with blanks", script: "((i = 2 + 3)); echo $i\n", want: "5\n"},
		{
			// The blanks and the star are data, because the expression goes to `let`
			// as one quoted word: unquoted, the star would list the directory.
			name: "a multiplication", script: "a=3\nb=4\n((c = a * b))\necho $c\n", want: "12\n",
		},
		{name: "a nested parenthesis", script: "((x = (1 + 2) * 3)); echo $x\n", want: "9\n"},
		{name: "as an if condition", script: "i=5\nif ((i < 10)); then echo less; fi\n", want: "less\n"},
		{
			name:   "as a while condition",
			script: "i=0\nwhile ((i < 3)); do i=$((i+1)); done\necho $i\n", want: "3\n",
		},
		{name: "in an and-or list", script: "((1)) && echo yes\n", want: "yes\n"},
		{name: "a failing one in an or list", script: "((0)) || echo fell-through\n", want: "fell-through\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if strings.Contains(stderr, "not found") || strings.Contains(stderr, "grouping") {
				t.Fatalf("stderr = %q -- it was not read as an arithmetic command", stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q (status %d, stderr %q)",
					test.script, stdout, test.want, status, stderr)
			}
		})
	}
}

// A construct spelled with a parenthesis has to be recognised by every scan that has
// an opinion about parentheses, and there are five. These are the forms each of them
// is really for, so a fifth layer learning about `((` cannot cost any of them.
//
// Two forms are deliberately absent, because they do not work and did not work before
// `((` was added -- checked by building the previous commit and running them against
// it, which is the only way to tell a regression from a hole that was already there:
//
//	case a in (a) echo matched ;; esac    prints nothing, where POSIX allows the
//	                                      open parenthesis on a case pattern
//	echo $(echo $((2*3)))                 syntax error: unexpected )
//
// Neither is asserted here, because a test that pins broken behaviour reads as though
// the behaviour were wanted. They are recorded in the commit that found them.
func TestArithmeticCommand_leavesTheOtherParenthesesAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a subshell", script: "(echo inside)\n", want: "inside\n"},
		{name: "a subshell in a subshell", script: "( (echo nested) )\n", want: "nested\n"},
		{name: "an arithmetic expansion", script: "echo $((1+2))\n", want: "3\n"},
		{name: "a command substitution", script: "echo $(echo sub)\n", want: "sub\n"},
		{name: "an array assignment", script: "a=(x y)\necho ${a[1]}\n", want: "y\n"},
		{name: "a function definition", script: "f() { echo fn; }\nf\n", want: "fn\n"},
		{
			name: "a subshell with an assignment inside", script: "(x=1; echo $x)\n", want: "1\n",
		},
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
		})
	}
}

// `++` and `--` were missing, and the prefix form was worse than missing: `++i` lexed
// as two unary pluses, so it returned the variable unchanged and reported nothing.
// `$((++i))` printed 0 for an i of 0.
func TestArithmetic_incrementsAndDecrements(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The postfix form's value is the one before the change; the variable
			// still ends up incremented.
			name: "postfix increment", script: "i=0\necho $((i++))\necho $i\n", want: "0\n1\n",
		},
		{name: "prefix increment", script: "i=0\necho $((++i))\necho $i\n", want: "1\n1\n"},
		{name: "postfix decrement", script: "i=5\necho $((i--))\necho $i\n", want: "5\n4\n"},
		{name: "prefix decrement", script: "i=5\necho $((--i))\necho $i\n", want: "4\n4\n"},
		{name: "increment in a larger expression", script: "i=1\necho $((i++ + 10))\n", want: "11\n"},
		{name: "an unset variable starts at zero", script: "echo $((n++))\necho $n\n", want: "0\n1\n"},
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
		})
	}
}

// The sign operators live one character away and must keep working.
func TestArithmetic_leavesTheSignOperatorsAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "addition", script: "echo $((1+2))\n", want: "3\n"},
		{name: "subtraction", script: "echo $((3-1))\n", want: "2\n"},
		{name: "unary minus", script: "echo $((-5))\n", want: "-5\n"},
		{name: "unary plus", script: "echo $((+5))\n", want: "5\n"},
		{name: "subtracting a negative", script: "echo $((2 - -3))\n", want: "5\n"},
		{name: "add-assign", script: "i=1\necho $((i+=2))\n", want: "3\n"},
		{name: "subtract-assign", script: "i=5\necho $((i-=2))\n", want: "3\n"},
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
		})
	}
}

// A prefix increment needs somewhere to put the result, so a literal has to be
// refused rather than silently discarded.
func TestArithmetic_refusesToIncrementSomethingThatIsNotAVariable(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "echo $((++5))\n")

	// Then
	if status == 0 {
		t.Fatalf("status = 0, want a failure; stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "needs a variable") {
		t.Fatalf("stderr = %q, want it to say what is wrong", stderr)
	}
}
