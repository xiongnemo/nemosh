package runtime

import (
	"bytes"
	"context"
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
