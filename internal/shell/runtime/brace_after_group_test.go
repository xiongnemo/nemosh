package runtime_test

import "testing"

// A group's close ends a command as a separator does, so a `}` may close a brace group right
// after a subshell's `)` or before one, and a reserved word after a group's `}` begins the next
// command, its brace a group's: busybox's ash_test compound, func6, group_in_braces and
// groups_and_keywords1 write each, and bash reads them too. After a substitution's `)` the
// `}` is still the word's, as there.
func TestRuntime_braceBesideAGroupClose(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"(exit 2); {(true)}\necho Zero:$?\n", "Zero:0\n"},
		{"{ f() ( echo $1; )}\nf 2\n", "2\n"},
		{"{ f()(echo $1)}\nf 3\n", "3\n"},
		{"echo 2; ({ :; })\n", "2\n"},
		{"({ echo a; })\n", "a\n"},
		{"if { echo foo; } then { echo bar; } fi\n", "foo\nbar\n"},
		{"while { echo foo; } do { echo bar; break; } done\n", "foo\nbar\n"},
		{"{ echo a | (cat)}\n", "a\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
	if _, status := runScriptCapturing("{ echo $(echo x)}\necho after\n"); status == 0 {
		t.Errorf("{ echo $(echo x)} ran; want the group still open, as both references have it")
	}
}
