package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// disown, logout and enable were `not found`, 127, which reads as a program to install, and a
// bashrc calls all three. Each answer here is bash 5.3's, measured; busybox has none of them.
//
// disown takes a job out of the table, so `jobs`, `wait` and %N no longer see it. The job is
// not ended: a job here outlives the shell whether or not it is disowned, since the shell
// sends no hangup when it exits, which is also why -h, which keeps the job and only spares it
// that hangup, has nothing further to do.
func TestRuntime_disownTakesAJobOutOfTheTable(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout, stderr string
		status                       int
	}{
		{name: "the current job", script: "sleep 1 &\ndisown\necho $?\njobs\n", stdout: "0\n"},
		{name: "by number", script: "sleep 1 &\ndisown %1\njobs\n"},
		{name: "by pid", script: "sleep 1 &\ndisown $!\njobs\n"},
		{name: "all of them", script: "sleep 1 &\nsleep 1 &\ndisown -a\njobs\n"},
		{name: "the running ones", script: "sleep 1 &\ndisown -r\njobs\n"},
		{name: "-h keeps the job", script: "sleep 1 &\ndisown -h\njobs\n", stdout: "Running"},
		{name: "no job", script: "disown\n", stderr: "disown: current: no such job", status: 1},
		{name: "a job there is not", script: "disown %3\n", stderr: "disown: %3: no such job", status: 1},
		{
			name: "an option there is not", script: "disown -x\n", status: 2,
			stderr: "disown: -x: invalid option\ndisown: usage: disown [-h] [-ar] [jobspec ... | pid ...]",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != test.status || !strings.Contains(stdout, test.stdout) || test.stdout == "" && stdout != "" && stdout != "0\n" {
				t.Fatalf("status %d, stdout %q, stderr %q; want %d and %q", status, stdout, stderr, test.status, test.stdout)
			}
			if !strings.Contains(stderr, test.stderr) || test.stderr == "" && stderr != "" {
				t.Fatalf("stderr = %q, want %q", stderr, test.stderr)
			}
		})
	}
}

// logout is exit in a login shell, and anywhere else an error that leaves the shell running.
func TestRuntime_logoutLeavesOnlyALoginShell(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "logout\necho after\n")
	if status != 0 || stdout != "after\n" || !strings.Contains(stderr, "logout: not login shell: use `exit'") {
		t.Fatalf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}

	var out bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &out, Stderr: &out})
	rt.MarkLoginShell()
	if status := rt.RunScript(context.Background(), "logout 3\necho after\n"); status != 3 || out.String() != "" {
		t.Fatalf("login shell: status %d, output %q, want 3 and nothing after", status, out.String())
	}
}

// enable lists the builtins, as bash lists them, and says which a name is not. Switching one
// off is refused, as is loading one: a builtin here is part of the shell, and a program of the
// same name is run by its path.
func TestRuntime_enableListsTheBuiltins(t *testing.T) {
	status, stdout, _ := runSetScript(t, "enable\n")
	for _, want := range []string{"enable .\nenable :\nenable [\n", "enable cd\n", "enable disown\n", "enable echo\n", "enable logout\n"} {
		if status != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("enable = %d, %q, missing %q", status, stdout, want)
		}
	}
	if strings.Contains(stdout, "enable fg\n") || strings.Contains(stdout, "enable ulimit\n") {
		t.Fatalf("enable lists a builtin this shell refuses:\n%s", stdout)
	}
	if status, stdout, _ := runSetScript(t, "enable -s\n"); status != 0 || !strings.HasPrefix(stdout, "enable .\nenable :\nenable break\n") || strings.Contains(stdout, "enable cd\n") {
		t.Fatalf("enable -s = %d, %q, want the special builtins", status, stdout)
	}
	for _, test := range []struct {
		script, stderr string
		status         int
	}{
		{"enable cd\n", "", 0},
		{"enable nosuch\n", "enable: nosuch: not a shell builtin", 1},
		{"enable -n echo\n", "enable: -n: not implemented", 2},
		{"enable -f lib.dll name\n", "enable: -f: not implemented", 2},
		{"enable -z\n", "enable: -z: invalid option\nenable: usage: enable [-a] [-dnps] [-f filename] [name ...]", 2},
	} {
		status, stdout, stderr := runSetScript(t, test.script)
		if status != test.status || stdout != "" || !strings.Contains(stderr, test.stderr) || test.stderr == "" && stderr != "" {
			t.Errorf("%q = %d, %q, %q; want %d and %q", test.script, status, stdout, stderr, test.status, test.stderr)
		}
	}
}
