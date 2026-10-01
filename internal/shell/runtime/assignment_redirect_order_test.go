package runtime_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runInTempDir runs script in a directory of its own, and answers its stdout and stderr.
func runInTempDir(t *testing.T, script string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(t.TempDir())+"'\n"+script+"\n")
	return stdout.String(), stderr.String()
}

// A simple command's words and redirections come in POSIX 2.9.1's order, as busybox's
// evalcommand takes them: the command name and its arguments, then the redirections, their
// targets expanded and the files opened, and then the assignments in front of the command,
// inside the redirections. The targets were expanded first and the redirections made last.
// Each answer is busybox-w32's, measured. bash agrees on the first three; it expands the
// assignments before it makes the redirections, so it answers 7 for the fourth and lets the
// assignments' errors through.
func TestRuntime_redirectionsComeBetweenArgumentsAndAssignments(t *testing.T) {
	for _, test := range []struct {
		script, stdout, stderr string
	}{
		{script: `i=0; echo $((i+=1)) $((i+=1)) > f$i; ls f*; cat f2`, stdout: "f2\n1 2\n"},
		{script: `x=1 >$(echo f; false); echo $?`, stdout: "1\n"},
		{script: `$(exit 4) >$(echo h; exit 6); echo $?`, stdout: "6\n"},
		{script: `x=$(exit 3) >$(echo g; exit 7); echo $?`, stdout: "3\n"},
		{script: `x=$(echo e >&2) 2>/dev/null; echo "[$x]"`, stdout: "[]\n"},
		{script: `x=$(echo e >&2) true 2>/dev/null; echo done`, stdout: "done\n"},
		{script: `f() { echo "in f [$y]"; }; y=$(echo inner >&2; echo v) f 2>/dev/null`, stdout: "in f [v]\n"},
		{script: `x=$(echo e >&2) export z=1 2>/dev/null; echo "[$z]"`, stdout: "[1]\n"},
		{script: `readonly x=1; x=2 2>/dev/null; echo "s=$?"`},
		{script: `set -x; x=$(echo v) true 2>/dev/null`, stderr: "+ x=v true\n"},
		{script: `x=$(echo out) >o.txt; echo "[$x]"; cat o.txt`, stdout: "[out]\n"},
	} {
		if stdout, stderr := runInTempDir(t, test.script); stdout != test.stdout || stderr != test.stderr {
			t.Errorf("%s\n got %q, %q\nwant %q, %q", test.script, stdout, stderr, test.stdout, test.stderr)
		}
	}
}

// A redirection that fails leaves the assignments in front of the command unexpanded and
// unmade, as busybox has it: x keeps its value, and the substitution never ran to say e.
func TestRuntime_aFailedRedirectionLeavesTheAssignmentsAlone(t *testing.T) {
	stdout, stderr := runInTempDir(t, `x=old; x=$(echo new; echo e >&2) < nonexistent; echo "[$x]"`)
	if stdout != "[old]\n" || strings.Contains("\n"+stderr, "\ne\n") || !strings.Contains(stderr, "nonexistent") {
		t.Errorf("got %q, %q; want [old], and the redirection's failure alone on stderr", stdout, stderr)
	}
}
