package runtime_test

import "testing"

// `for name; do ... done` -- the list is the positional parameters, which POSIX
// 2.9.4.2 specifies and which is how a function loops over its arguments. It reported
// the usage message instead. And `for name in; do ... done`, an empty list, was the same
// error where it should simply run zero times -- which is what a generated list that
// came out empty looks like.
func TestPosixFor_loopsOverTheArguments(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "over the positional parameters",
			script: "set -- a b\nfor i; do printf '<%s>' \"$i\"; done\necho\n", want: "<a><b>\n",
		},
		{
			// An argument holding a blank stays one iteration: the values are already
			// the parameters and are not expanded again.
			name:   "an argument with a blank",
			script: "set -- \"a b\" c\nfor i; do printf '<%s>' \"$i\"; done\necho\n", want: "<a b><c>\n",
		},
		{
			name:   "inside a function",
			script: "f() { for i; do printf '<%s>' \"$i\"; done; echo; }\nf p q\n", want: "<p><q>\n",
		},
		{name: "with no arguments at all", script: "set --\nfor i; do echo never; done\necho done\n", want: "done\n"},
		{name: "break out of it", script: "set -- a b c\nfor i; do break; done\necho \"$i\"\n", want: "a\n"},
		{
			name:   "continue in it",
			script: "set -- a b\nfor i; do continue; printf never; done\necho done\n", want: "done\n",
		},
		// The empty list.
		{name: "an empty in list", script: "for i in; do echo never; done\necho done\n", want: "done\n"},
		{
			name:   "an empty list from an expansion",
			script: "set --\nfor i in \"$@\"; do echo never; done\necho done\n", want: "done\n",
		},
		// The ordinary forms, unchanged.
		{name: "a word list", script: "for i in a b; do printf '<%s>' \"$i\"; done\necho\n", want: "<a><b>\n"},
		{name: "the counted form", script: "for ((i=0;i<2;i++)); do printf %s $i; done\necho\n", want: "01\n"},
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

// bash's `base#digits`: how a script reads a binary mask or a hex value without a
// leading 0x. The `#` ended the word and became a token of its own, reported as
// `unexpected "#"`.
func TestArithmeticBase_readsAnExplicitRadix(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "binary", script: "echo $((2#101))\n", want: "5\n"},
		{name: "hex", script: "echo $((16#ff))\n", want: "255\n"},
		{name: "octal", script: "echo $((8#17))\n", want: "15\n"},
		{name: "base 36", script: "echo $((36#z))\n", want: "35\n"},
		{name: "in an expression", script: "echo $((2#101 + 1))\n", want: "6\n"},
		// The C spellings, which must keep working.
		{name: "the 0x prefix", script: "echo $((0x10))\n", want: "16\n"},
		{name: "a leading zero is octal", script: "echo $((010))\n", want: "8\n"},
		{name: "plain decimal", script: "echo $((5))\n", want: "5\n"},
		{name: "a variable", script: "x=3\necho $((x*2))\n", want: "6\n"},
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

// A redirection after a closer may name its descriptor -- `done 2>/dev/null` -- which is
// how a loop's diagnostics are silenced. The suffix scan wanted `<` or `>` at the start
// and a digit is not one.
func TestCompoundRedirect_acceptsADescriptorNumber(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"while getopts a o; do :; done 2>/dev/null\necho ok\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "ok\n" {
		t.Fatalf("stdout = %q, want ok", stdout)
	}
}
