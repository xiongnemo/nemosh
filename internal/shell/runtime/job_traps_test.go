package runtime

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A signal the shell ignores stays ignored in a background job, under both launchers: the job
// lists it, as both references' do, and a TERM sent to the job is ignored, as bash ignores it.
// busybox-w32 ends the job, having no way to deliver a signal to one (signal_job_test.go). A trap
// that runs something is the shell's alone, as before.
func TestJobTraps_keepWhatTheShellIgnores(t *testing.T) {
	const ready = "until [ -e \"$d/ready\" ]; do sleep 0.02; done\n"
	for _, test := range []struct {
		name, script, stdout string
	}{
		{
			name:   "listed",
			script: "trap '' INT\ntrap 'echo parent-term' TERM\n{ trap; } & wait\n",
			stdout: "trap -- '' INT\n",
		},
		{
			name:   "a TERM sent to the job is ignored",
			script: "trap '' TERM\n( : > \"$d/ready\"; sleep 1; echo after ) &\n" + ready + "kill $!; wait $!; echo \"st=$?\"\n",
			stdout: "after\nst=0\n",
		},
	} {
		for _, launcher := range []string{"goroutine", "process"} {
			t.Run(launcher+": "+test.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
				rt.env.Set("NEMOSH_JOBS", launcher)
				script := "d='" + filepath.ToSlash(t.TempDir()) + "'\n" + test.script
				status := rt.RunScript(context.Background(), script)
				rt.CloseBatch(status)
				if stdout.String() != test.stdout || strings.Contains(stderr.String(), "Terminated") {
					t.Fatalf("stdout %q, want %q; stderr %q", stdout.String(), test.stdout, stderr.String())
				}
			})
		}
	}
}

// In bash mode a job lists the shell's other traps too, without running them, as bash's does,
// and under `set -E` runs its ERR trap for the commands inside it, though not for the job's own
// failure, as bash's does. busybox's job lists only what it ignores and runs no ERR trap, which
// nemosh and sh keep. Measured 2026-11-22 against Git for Windows' bash 5.3 and busybox-w32.
func TestJobTraps_bashModeListsTheShellsAndRunsErrUnderErrtrace(t *testing.T) {
	// FUNCNAME rather than LINENO says where the trap ran: a job process numbers a function's
	// lines from its printed text.
	const errtrace = "set -o errtrace\ntrap 'echo ERR ${FUNCNAME:-main}' ERR\nfalse & wait\n{ false; } & wait\ng() { false; }\ng & wait\n" +
		"if false; then :; else false; fi & wait\nfalse | false & wait\n"
	for _, test := range []struct {
		name, script, bash, nemosh string
	}{
		{
			name:   "listed, and not run",
			script: "trap 'echo parent-exit' EXIT\ntrap 'echo parent-term' TERM\n{ trap; } & wait\necho between\n",
			bash:   "trap -- 'echo parent-exit' EXIT\ntrap -- 'echo parent-term' SIGTERM\nbetween\nparent-exit\n",
			nemosh: "between\nparent-exit\n",
		},
		{name: "ERR under errtrace", script: errtrace, bash: "ERR main\nERR g\nERR main\n"},
	} {
		for _, launcher := range []string{"goroutine", "process"} {
			for _, bash := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s: %s, bash mode %v", launcher, test.name, bash), func(t *testing.T) {
					var stdout, stderr bytes.Buffer
					rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
					rt.env.Set("NEMOSH_JOBS", launcher)
					want := test.nemosh
					if bash {
						rt.EnterBashMode()
						want = test.bash
					}
					status := rt.RunScript(context.Background(), test.script)
					rt.CloseBatch(status)
					if stdout.String() != want {
						t.Fatalf("stdout %q, want %q; stderr %q", stdout.String(), want, stderr.String())
					}
				})
			}
		}
	}
}
