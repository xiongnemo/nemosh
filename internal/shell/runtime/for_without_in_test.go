package runtime_test

import "testing"

// `for name do` with no `in` and no `;` loops over "$@", as POSIX's grammar has it
// (for_clause: For name do_group) and busybox-w32 and bash both run it. The `do` shared the
// header's segment, so the loop was "done before do".
func TestRuntime_forWithoutInOrSeparator(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"set -- a b\nfor x do\n  echo hi\n  echo $x\ndone\n", "hi\na\nhi\nb\n"},
		{"set -- a b\nfor x do echo $x; done\n", "a\nb\n"},
		{"f() { for x do echo \"<$x>\"; done; }; f 1 '2 3'\n", "<1>\n<2 3>\n"},
		{"set -- a\nfor x\tdo echo $x; done\n", "a\n"},
		// Anything else after the name is still a word list or an error, as before.
		{"set -- a b\nfor x in do; do echo $x; done\n", "do\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
