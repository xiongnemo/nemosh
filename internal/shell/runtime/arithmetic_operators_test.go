package runtime_test

import "testing"

// Found by a differential sweep against bash rather than by reading: `**` and `,`
// reported `arithmetic syntax error`, and `$$` and `${#1}` were an empty string and a
// `bad substitution`.
func TestArithmeticOperators_powerAndComma(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "power", script: "echo $((2**10))\n", want: "1024\n"},
		{
			// Right-associative: 2**(3**2) is 512, not (2**3)**2 which is 64. The
			// ordinary precedence table folds left, so this needed its own level.
			name: "power is right-associative", script: "echo $((2**3**2))\n", want: "512\n",
		},
		{
			// It binds tighter than unary minus, so the minus applies to the result.
			name: "power binds tighter than unary minus", script: "echo $((-2**2))\n", want: "4\n",
		},
		{name: "power of zero", script: "echo $((5**0))\n", want: "1\n"},
		{name: "power with a variable", script: "n=3\necho $((2**n))\n", want: "8\n"},
		{name: "comma", script: "echo $((1,2))\n", want: "2\n"},
		{name: "comma three times", script: "echo $((1,2,3))\n", want: "3\n"},
		{
			// Comma is outside assignment, so both sides take effect and the last is
			// the value.
			name: "comma with assignments", script: "echo $((a=1, b=2))\necho \"$a$b\"\n", want: "2\n12\n",
		},
		{name: "comma in a for step", script: "for ((i=0;i<2;i++)); do printf %s $i; done\necho\n", want: "01\n"},
		// The operators either side of the new ones, unchanged.
		{name: "multiplication", script: "echo $((2*3))\n", want: "6\n"},
		{name: "addition", script: "echo $((1+2))\n", want: "3\n"},
		{name: "precedence", script: "echo $((2+3*4))\n", want: "14\n"},
		{name: "parentheses", script: "echo $(((2+3)*4))\n", want: "20\n"},
		{name: "increment", script: "i=0\necho $((i++))\necho $i\n", want: "0\n1\n"},
		{name: "shift", script: "echo $((1<<4))\n", want: "16\n"},
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

// A negative exponent has no integer answer, so it is refused rather than rounded to
// something.
func TestArithmeticOperators_refusesANegativeExponent(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "echo $((2**-1))\n")

	// Then
	if status == 0 {
		t.Fatalf("status = 0, want a failure; stderr = %q", stderr)
	}
	if !contains(stderr, "exponent less than 0") {
		t.Fatalf("stderr = %q, want it to say why", stderr)
	}
}

// `$$` is how a script names a temporary file. It was empty, so `/tmp/work.$$` was
// `/tmp/work.` for every run and two scripts collided on it.
func TestSpecialParameters_processId(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "not empty", script: "test -n \"$$\" && echo yes\n", want: "yes\n"},
		{name: "all digits", script: "case \"$$\" in *[!0-9]*) echo no ;; *) echo digits ;; esac\n", want: "digits\n"},
		{name: "stable within a run", script: "a=$$\nb=$$\ntest \"$a\" = \"$b\" && echo stable\n", want: "stable\n"},
		{
			name:   "usable in a filename",
			script: "x=/tmp/w.$$\ncase \"$x\" in */w.[0-9]*) echo named ;; esac\n", want: "named\n",
		},
		{name: "braced", script: "test -n \"${$}\" && echo yes\n", want: "yes\n"},
		{name: "its length", script: "test \"${#$}\" -gt 0 && echo yes\n", want: "yes\n"},
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

// `${#1}` is how a script checks the size of an argument, and it reported
// `bad substitution` -- the length form only accepted a variable name.
func TestSpecialParameters_lengthOfAPositional(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "first argument", script: "set -- aa bbb\necho ${#1}\n", want: "2\n"},
		{name: "second argument", script: "set -- aa bbb\necho ${#2}\n", want: "3\n"},
		{name: "an absent argument", script: "set -- a\necho ${#9}\n", want: "0\n"},
		{name: "the status", script: "true\necho ${#?}\n", want: "1\n"},
		{name: "the count of arguments", script: "set -- a b c\necho ${#@}\n", want: "3\n"},
		// The ordinary form, unchanged.
		{name: "a variable", script: "x=abcd\necho ${#x}\n", want: "4\n"},
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
