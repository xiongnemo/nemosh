package runtime_test

import "testing"

// A function definition is a command, and may be an if's or a loop's condition, as busybox and
// bash read it: it defines the function and succeeds. The condition's header went to the line
// parser, which has no definitions, and was "unexpected )". busybox's ash_test func7.
func TestRuntime_aFunctionDefinitionMayBeACondition(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"if f() { echo Ok:$?; } then f; fi\n", "Ok:0\n"},
		{"if f() { echo Ok:$?; }; then f; fi\n", "Ok:0\n"},
		{"while f() { echo W; }; do f; break; done\n", "W\n"},
		{"if g() ( echo sub ); then g; fi\n", "sub\n"},
		{"until h() { echo H; }; do :; done; h\n", "H\n"},
		{"if f() { echo x; } && false; then echo no; else echo else; fi\n", "else\n"},
		{"if [ \"$(echo a)\" = a ]; then echo paren; fi\n", "paren\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
