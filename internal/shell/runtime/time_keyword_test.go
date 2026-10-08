package runtime_test

import (
	"strings"
	"testing"
)

// bash's time keyword times a compound command, which busybox's time command cannot: a brace
// group in the shell itself, a subshell, a loop, an if, a case, one written over lines, and one
// with the rest of its pipeline. Each was a syntax error, and `time ( ... )` a function named
// time. A time before a simple command is still the command. The report is $TIME's, here the
// status alone, so it can be compared.
func TestTimeKeyword_timesACompoundCommand(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout, stderr string
	}{
		{name: "brace group", script: "time { echo a; false; }; echo \"st=$?\"",
			stdout: "a\nst=1\n", stderr: "Command exited with non-zero status 1\ntook 1\n"},
		{name: "the group is the shell's", script: "x=1; time { x=2; }; echo \"x=$x\"",
			stdout: "x=2\n", stderr: "took 0\n"},
		{name: "subshell", script: "time ( echo sub )", stdout: "sub\n", stderr: "took 0\n"},
		{name: "over lines", script: "time {\n  echo one\n  echo two\n}\necho after",
			stdout: "one\ntwo\nafter\n", stderr: "took 0\n"},
		{name: "loop", script: "time for i in 1 2; do echo $i; done", stdout: "1\n2\n", stderr: "took 0\n"},
		{name: "if", script: "time if true; then echo yes; fi", stdout: "yes\n", stderr: "took 0\n"},
		{name: "case", script: "time case x in x) echo arm;; esac", stdout: "arm\n", stderr: "took 0\n"},
		{name: "the whole pipeline", script: "time { echo a; echo b; } | sort -r", stdout: "b\na\n", stderr: "took 0\n"},
		{name: "a simple command is the command", script: "time echo plain", stdout: "plain\n", stderr: "took 0\n"},
		{name: "a brace after a word is a word", script: "echo time { word", stdout: "time { word\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, "export TIME='took %x'\n"+test.script+"\n")

			// Then
			if status != 0 || stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("status %d, stdout %q, stderr %q; want 0, %q and %q", status, stdout, stderr, test.stdout, test.stderr)
			}
		})
	}
}

// -p is POSIX's form, real, user and sys in seconds, as `time -p CMD` gives it.
func TestTimeKeyword_takesPForPOSIXForm(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "time -p { echo p; }\n")
	if status != 0 || stdout != "p\n" || !strings.HasPrefix(stderr, "real ") || !strings.Contains(stderr, "\nuser ") {
		t.Fatalf("status %d, stdout %q, stderr %q; want p and a POSIX report", status, stdout, stderr)
	}
}
