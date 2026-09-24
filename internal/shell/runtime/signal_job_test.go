package runtime

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A signal `kill` sends a job, under both launchers: the goroutine, and the job process of
// NEMOSH_JOBS=process. Each answer is bash 5.3's, measured; busybox-w32 ends the job
// whatever it has trapped, because it has no way to deliver a signal to one. Every job says
// it is ready once its trap is set, so the kill never races the trap.
func TestSignalledJobs_runTheirTrapsAsBashDoes(t *testing.T) {
	const ready = "until [ -e \"$d/ready\" ]; do sleep 0.02; done\n"
	for _, test := range []struct {
		name, script, stdout, stderr string
	}{
		{
			name:   "a trapped TERM runs the trap, and the job carries on",
			script: "( trap 'echo got-term' TERM; : > \"$d/ready\"; sleep 1; echo after ) &\n" + ready + "kill $!; wait $!; echo \"st=$?\"\n",
			stdout: "got-term\nafter\nst=0\n",
		},
		{
			name:   "in a group too",
			script: "{ trap 'echo got-hup' HUP; : > \"$d/ready\"; sleep 1; echo after; } &\n" + ready + "kill -HUP $!; wait $!; echo \"st=$?\"\n",
			stdout: "got-hup\nafter\nst=0\n",
		},
		{
			name:   "an untrapped TERM ends the job, and its EXIT trap runs",
			script: "( trap 'echo exit-trap' EXIT; : > \"$d/ready\"; sleep 5; echo after ) &\n" + ready + "kill $!; wait $!; echo \"st=$?\"\n",
			stdout: "exit-trap\nst=143\n",
			stderr: "Terminated",
		},
		{
			name:   "an ignored TERM does nothing",
			script: "( trap '' TERM; : > \"$d/ready\"; sleep 1; echo after ) &\n" + ready + "kill $!; wait $!; echo \"st=$?\"\n",
			stdout: "after\nst=0\n",
		},
		{
			name:   "a trap can end the job, between the commands of a loop",
			script: "( trap 'echo got-term; exit 3' TERM; : > \"$d/ready\"; while :; do :; done ) &\n" + ready + "kill $!; wait $!; echo \"st=$?\"\n",
			stdout: "got-term\nst=3\n",
		},
		{
			name:   "KILL is not caught",
			script: "( trap 'echo got' TERM; : > \"$d/ready\"; sleep 5 ) &\n" + ready + "kill -9 $!; wait $!; echo \"st=$?\"\n",
			stdout: "st=137\n",
			stderr: "Killed",
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
				if stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.stderr) {
					t.Fatalf("stdout %q, want %q\nstderr %q, want it to contain %q", stdout.String(), test.stdout, stderr.String(), test.stderr)
				}
			})
		}
	}
}

// `trap -p` shows a TERM trap as it shows any other, so saving and restoring the table
// keeps it.
func TestSignalTraps_areListedAndReset(t *testing.T) {
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	status := rt.RunScript(context.Background(), "trap 'echo t' TERM HUP QUIT\ntrap -p TERM\ntrap - TERM\ntrap\n")
	want := "trap -- 'echo t' TERM\ntrap -- 'echo t' HUP\ntrap -- 'echo t' QUIT\n"
	if status != 0 || stdout.String() != want {
		t.Fatalf("status %d, stdout %q, want %q", status, stdout.String(), want)
	}
}
