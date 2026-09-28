package runtime_test

import "testing"

// A command abandoned at the top level -- a pattern that matched nothing under failglob, an
// assignment that could not be made -- takes the rest of its line with it, as bash has it,
// and the script goes on with the next line. The commands after it on the line ran.
func TestRuntime_discardTakesTheRestOfTheLine(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"shopt -s failglob\necho *.ZZ; echo same=$?\necho *.ZZ\necho next=$?\n", "next=1\n"},
		{"a=(); a[-1]=1; echo same\necho \"next $?\"\n", "next 1\n"},
		{"a=()\na[-1]=1\necho \"next $?\"\n", "next 1\n"},
		{"a=(); a[-1]=1 || echo or; echo same\nfor i in 1; do echo loop; done\n", "loop\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
