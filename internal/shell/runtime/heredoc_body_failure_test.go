package runtime_test

import "testing"

// An expansion that fails in a heredoc's body fails that redirection, and the command with it,
// not the script, as in busybox-w32 and bash 5.3: the command does not run, leaves 1, and the
// script goes on. A special builtin's failed redirection still ends the script, with 1. Each
// ended the script, with 2.
func TestHeredoc_aFailedExpansionInTheBodyFailsTheCommand(t *testing.T) {
	for _, test := range []struct{ name, script, stdout, stderr string }{
		{name: "a simple command", script: "cat <<EOF\n${D?unset}\nEOF\necho \"after $?\"", stdout: "after 1\n", stderr: "nemosh: line 1: D: unset\n"},
		{name: "on the same line", script: "cat <<EOF; echo \"same $?\"\n${D?unset}\nEOF\necho \"after $?\"", stdout: "same 1\nafter 0\n", stderr: "nemosh: line 1: D: unset\n"},
		{name: "or a handler", script: "cat <<EOF || echo \"or $?\"\n$((1/0))\nEOF", stdout: "or 1\n", stderr: "nemosh: line 1: divide by zero\n"},
		{name: "a group", script: "{ cat; } <<EOF\n${D?unset}\nEOF\necho \"after $?\"", stdout: "after 1\n", stderr: "nemosh: line 1: D: unset\n"},
		{name: "a loop", script: "while read l; do echo \"got $l\"; done <<EOF\na\n${D?unset}\nEOF\necho \"after $?\"", stdout: "after 1\n", stderr: "nemosh: line 1: D: unset\n"},
		{name: "set -u", script: "set -u\ncat <<EOF\n$undefined\nEOF\necho \"after $?\"", stdout: "after 1\n", stderr: "nemosh: line 2: undefined: parameter not set\n"},
		{name: "in a substitution", script: "x=$(cat <<EOF\n${D?unset}\nEOF\n)\necho \"after $? [$x]\"", stdout: "after 1 []\n", stderr: "nemosh: line 1: D: unset\n"},
		{name: "a special builtin's ends the script", script: ": <<EOF\n${D?unset}\nEOF\necho after", stdout: "", stderr: "nemosh: line 1: D: unset\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script+"\n")
			if stdout != test.stdout || stderr != test.stderr {
				t.Fatalf("stdout = %q, stderr = %q, want %q and %q", stdout, stderr, test.stdout, test.stderr)
			}
			if test.name == "a special builtin's ends the script" && status != 1 {
				t.Fatalf("status = %d, want 1", status)
			}
		})
	}
}
