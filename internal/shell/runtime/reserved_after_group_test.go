package runtime_test

import "testing"

// A reserved word may follow a brace group's `}` or a subshell's `)` directly, with no
// separator between: the group is a complete command, so what comes next is a reserved word
// where one may stand, as POSIX has it and busybox-w32 and bash both read it. `if { true; }
// then` was "fi before then", and `{ echo g; } fi` never closed its if. `(( ))` is bash's,
// and bash reads it the same way.
func TestRuntime_aReservedWordMayFollowAGroup(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"if { true; } then echo yes; fi", "yes\n"},
		{"while { false; } do echo x; done; echo ok", "ok\n"},
		{"if ( true ) then echo sub; fi", "sub\n"},
		{"if true; then { echo g; } fi", "g\n"},
		{"for i in 1; do { echo $i; } done", "1\n"},
		{"set -o errexit; if { ! false; false; true; } then echo true; fi", "true\n"},
		{"if false; then :; elif { true; } then echo elif; fi", "elif\n"},
		{"if false; then :; else { echo e; } fi", "e\n"},
		{"case x in x) { echo c; } ;; esac", "c\n"},
		{"f() { { echo in; } }; f", "in\n"},
		{"if (( 1 )) then echo arith; fi", "arith\n"},
		// And a `}` may follow the end of a compound: the fi, done or esac, where a reserved
		// word may stand, though not after one that is only an argument.
		{"{ if true; then echo a; fi }", "a\n"},
		{"{ for i in 1; do echo $i; done }", "1\n"},
		{"{ case x in x) echo c;; esac }", "c\n"},
		{"echo done }", "done }\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, test.want)
			}
		})
	}
}
