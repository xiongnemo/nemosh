package runtime_test

import "testing"

// An `elif` may end its line, its condition on the lines after it, as `if` may: both
// references read it, and busybox's ash_test assignment1 writes it so. It was `duplicate
// then`. A bare `while` inside an if keeps the if's elif chain its own too.
func TestRuntime_elifMayEndItsLine(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"if true; then echo a; elif\n true; then echo b; fi\n", "a\n"},
		{"if false; then echo a; elif\n true; then echo b; fi\n", "b\n"},
		{"if\n false; then echo a; elif\n false; then echo b; else\n echo c; fi\n", "c\n"},
		{"if true; then\n  while\n    false\n  do :; done\n  echo w\nelif true; then echo e; fi\n", "w\n"},
		{"if false; then :; elif false; then :; elif\ntrue\nthen echo third; fi\n", "third\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
