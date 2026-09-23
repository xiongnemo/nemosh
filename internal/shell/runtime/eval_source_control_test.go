package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `eval` and `.` run text as part of the current shell, so what that text does to the
// shell's flow, it does to the caller's. They kept the status and dropped the rest: an
// `exit` in a sourced file did not exit, and `eval break` did not break. Each answer here is
// busybox-w32's, measured.
func TestEvalAndSource_carryTheirControlOut(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sh")
	if err := os.WriteFile(lib, []byte("exit 4\necho not-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	returns := filepath.Join(dir, "returns.sh")
	if err := os.WriteFile(returns, []byte("return 6\necho not-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slash := func(path string) string { return filepath.ToSlash(path) }
	for _, test := range []struct {
		name   string
		script string
		stdout string
		status int
	}{
		{name: "exit in eval exits", script: "eval 'exit 3'\necho after\n", status: 3},
		{name: "exit in a sourced file exits", script: ". " + slash(lib) + "\necho after\n", status: 4},
		{name: "source is the same builtin", script: "source " + slash(lib) + "\necho after\n", status: 4},
		{name: "break in eval leaves the loop", script: "for i in 1 2 3; do eval break; echo $i; done\necho done\n", stdout: "done\n"},
		{name: "continue in eval", script: "for i in 1 2; do eval continue; echo $i; done\necho done\n", stdout: "done\n"},
		{name: "return in eval returns from the function", script: "f() { eval 'return 5'; echo no; }\nf\necho \"st=$?\"\n", stdout: "st=5\n"},
		// The one control `.` consumes: return ends the sourced file, not the shell.
		{name: "return in a sourced file ends only the file", script: ". " + slash(returns) + "\necho \"st=$?\"\n", stdout: "st=6\n"},
		{name: "a shell error in eval aborts", script: "set -u\neval 'echo $nope'\necho after\n", status: 2},
		// Redirections still apply to both, and a leading assignment persists, as it does
		// for any special builtin (POSIX 2.9.1).
		{name: "redirection on eval", script: "eval 'echo in' >/dev/null\necho out\n", stdout: "out\n"},
		{name: "assignment before eval persists", script: "x=1 eval 'echo $x'\necho \"x=$x\"\n", stdout: "1\nx=1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
			status := rt.RunScript(context.Background(), test.script)
			if stdout.String() != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d (stderr %q)", stdout.String(), status, test.stdout, test.status, stderr.String())
			}
		})
	}
}

// A shell error typed at a prompt ends the line, not the session. POSIX: an interactive
// shell "shall write a diagnostic message to standard error without exiting". `set -u;
// echo $nope` used to close the terminal it was typed into.
func TestInteractive_aShellErrorDoesNotEndTheSession(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	for _, line := range []string{"set -u\n", "echo $nope; echo same-line\n"} {
		script, err := ParseScript(line)
		if err != nil {
			t.Fatal(err)
		}
		if result := rt.RunInteractive(context.Background(), script); result.Exited {
			t.Fatalf("%q closed the session (stderr %q)", line, stderr.String())
		}
	}
	script, _ := ParseScript("echo next-line\n")
	result := rt.RunInteractive(context.Background(), script)
	if result.Exited || stdout.String() != "next-line\n" {
		t.Fatalf("stdout = %q, exited = %v; want the rest of the erring line abandoned and the next one run",
			stdout.String(), result.Exited)
	}
	if !strings.Contains(stderr.String(), "nope: parameter not set") {
		t.Fatalf("stderr = %q, want the diagnostic", stderr.String())
	}
}
