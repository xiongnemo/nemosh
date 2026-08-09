package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runWhich(t *testing.T, script string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
	})
	status := rt.RunScript(context.Background(), script)
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing", stderr.String())
	}
	return stdout.String(), status
}

// `which` is a builtin rather than an applet so that its answer is the shell's
// own lookup, and cannot disagree with what typing the name would run. An applet
// would have needed a second copy of PATH search and the Windows suffix order,
// and two copies drift -- which is the defect `command -v` was fixed for, when it
// said nothing about a program that plain dispatch ran perfectly well.
func TestWhich_reportsWhatTheShellWouldRun(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
		status int
	}{
		{name: "a builtin by name", script: "which cd\n", want: "cd\n"},
		{name: "an applet by name", script: "which echo\n", want: "echo\n"},
		{name: "several at once", script: "which cd echo\n", want: "cd\necho\n"},
		{name: "nothing to find is silent", script: "which zzznosuchprogram\n", want: "", status: 1},
		{name: "no operand at all", script: "which\n", want: "", status: 1},
		// One bad name among good ones still reports the good ones, and still
		// fails: a script testing the status must not be told everything is fine.
		{name: "a mixture", script: "which cd zzznosuchprogram\n", want: "cd\n", status: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, status := runWhich(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
			if status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
		})
	}
}

// And it agrees with `command -v`, which is the same lookup asked a different
// way. If these two ever differ, one of them is lying about what will run.
func TestWhich_agreesWithCommandV(t *testing.T) {
	for _, name := range []string{"cd", "echo", "ls", "zzznosuchprogram"} {
		t.Run(name, func(t *testing.T) {
			fromWhich, whichStatus := runWhich(t, "which "+name+"\n")
			fromCommand, commandStatus := runWhich(t, "command -v "+name+"\n")
			if fromWhich != fromCommand || whichStatus != commandStatus {
				t.Fatalf("which says %q (%d) and command -v says %q (%d)",
					fromWhich, whichStatus, fromCommand, commandStatus)
			}
		})
	}
}
