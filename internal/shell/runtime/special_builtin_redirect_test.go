package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// A special builtin whose redirection fails ends the script, status 1, as POSIX 2.8.1 has it
// and busybox-w32 does -- even under `||` or in an if, and a subshell ends alone. `command`
// takes the special away, and an ordinary command, a function or an assignment alone goes on
// with status 1, as before. The error was said and the script went on.
func TestRuntime_specialBuiltinsFailedRedirectionEndsTheScript(t *testing.T) {
	bad := filepath.ToSlash(filepath.Join(t.TempDir(), "no", "such", "file"))
	for _, test := range []struct {
		script, want string
		status       int
	}{
		{": > BAD; echo after", "", 1},
		{"export abc=def > BAD; echo after", "", 1},
		{"eval 'echo hi' > BAD; echo after", "", 1},
		{". /dev/null > BAD; echo after", "", 1},
		{": > BAD || echo handled; echo after", "", 1},
		{"if : > BAD; then :; fi; echo after", "", 1},
		{"exec > BAD; echo after", "", 1},
		{"exec 3< BAD; echo after", "", 1},
		{"( : > BAD ); echo \"after $?\"", "after 1\n", 0},
		{"command : > BAD; echo \"after $?\"", "after 1\n", 0},
		{"echo hi > BAD; echo \"after $?\"", "after 1\n", 0},
		{"x=1 > BAD; echo \"after $?\"", "after 1\n", 0},
		{"f() { :; }; f > BAD; echo \"after $?\"", "after 1\n", 0},
		// A descriptor made a copy of itself is left as it is, open or not, so nothing
		// fails: `3>&3` with 3 closed, in both references, and bash's `3>&3-`, which
		// busybox-w32 has not got and refuses as a redir error.
		{": 3>&3; echo hello; : 3>&3-; echo again", "hello\nagain\n", 0},
	} {
		script := strings.ReplaceAll(test.script, "BAD", bad)
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script + "\n"); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as busybox-w32 answers", stdout, status, test.want, test.status)
			}
		})
	}
}
