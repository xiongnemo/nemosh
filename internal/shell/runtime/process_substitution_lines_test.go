package runtime_test

import "testing"

// A process substitution's command may go on past its line, as a command substitution's may,
// and a parenthesis quoted inside it is its text: both references read each of these. `done <
// <(` with the command on the lines after it was `<: missing redirection target`, a `\` ending
// one of its lines was kept, and the `)` in `<(echo "a )")` ended it. So did a quoted one inside
// an extended pattern. Every answer is bash's; busybox has no process substitution.
func TestRuntime_processSubstitutionSpansLines(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"while read x; do echo \"got $x\"; done < <(\n  echo one\n  echo two\n)\n", "got one\ngot two\n"},
		{"cat <(echo a\necho b)\n", "a\nb\n"},
		{"cat <(echo a \\\nb)\n", "a b\n"},
		{"diff <(echo a) <(\necho a\n) && echo same\n", "same\n"},
		{"f() {\n\twhile read x; do\n\t\techo \"[$x]\"\n\tdone < <(printf \"%s\\n\" a \\\n\t\tb 2>/dev/null)\n}\nf\n", "[a]\n[b]\n"},
		{"cat <(echo \"a )\"\necho \"b (\")\n", "a )\nb (\n"},
		{"x=$(cat <(echo in\necho sub)); echo \"$x\"\n", "in\nsub\n"},
		{"cat <(\nif true; then echo yes; fi\n)\n", "yes\n"},
		{"cat <(echo \"a )\")\n", "a )\n"},
		{"f() { cat <(echo \"a )\"); }; f\n", "a )\n"},
		{"if true; then cat <(echo \"a )\"); fi\n", "a )\n"},
		{"shopt -s extglob; case \"a)\" in @(\"a)\"|b)) echo m;; *) echo n;; esac\n", "m\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
