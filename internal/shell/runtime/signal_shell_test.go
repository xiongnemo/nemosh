package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// EndJobs, which cmd/nemosh calls when Ctrl-C has ended a script, ends a job that is still
// running under either launcher, the process's programs with it, and returns once it has.
func TestEndJobs_endsWhatIsStillRunning(t *testing.T) {
	for _, launcher := range []string{"goroutine", "process"} {
		t.Run(launcher, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
			rt.env.Set("NEMOSH_JOBS", launcher)
			rt.RunScript(context.Background(), "sleep 30 & p=$!\n")
			started := time.Now()
			rt.EndJobs()
			if waited := time.Since(started); waited > 10*time.Second {
				t.Fatalf("EndJobs took %v", waited)
			}
			rt.RunScript(context.Background(), "kill -0 $p 2>/dev/null; echo \"asked=$?\"\n")
			if stdout.String() != "asked=1\n" {
				t.Fatalf("stdout %q: the job is still there", stdout.String())
			}
		})
	}
}

// A script sent a signal from outside it -- what cmd/nemosh hands ReceiveSignals -- gets
// bash 5.3's answers, measured with a script bash ran and `kill` sent TERM.
func TestReceiveSignals_aScriptGetsBashsAnswers(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout string
		status, signal       int
		final                bool
	}{
		{
			name:   "a trapped TERM runs the trap, and the script carries on",
			script: "trap 'echo got-term' TERM\n: > \"$d/ready\"\nsleep 1\necho after\n",
			stdout: "got-term\nafter\n",
		},
		{
			name:   "an untrapped TERM ends the script, and its EXIT trap runs",
			script: "trap 'echo bye' EXIT\n: > \"$d/ready\"\nsleep 5\necho after\n",
			stdout: "bye\n", status: 143, signal: 15,
		},
		{
			name:   "the trap can end the script",
			script: "trap 'echo got-term; exit 4' TERM\n: > \"$d/ready\"\nwhile :; do :; done\n",
			stdout: "got-term\n", status: 4,
		},
		// A console closing, on Windows: the command in progress does not get to finish.
		{
			name:   "a final TERM ends the script, and runs its trap and then its EXIT trap",
			script: "trap 'echo got-term' TERM\ntrap 'echo bye' EXIT\n: > \"$d/ready\"\nsleep 20\necho after\n",
			stdout: "got-term\nbye\n", status: 143, signal: 15, final: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			var stdout bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
			signals := make(chan int, 1)
			ctx, stop := rt.ReceiveSignals(context.Background(), signals, test.final)
			go func() {
				for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
					if _, err := os.Stat(filepath.Join(directory, "ready")); err == nil {
						signals <- 15
						return
					}
				}
			}()
			status := rt.RunScript(ctx, "d='"+filepath.ToSlash(directory)+"'\n"+test.script)
			stop()
			signal, _ := ExitSignal(ctx)
			if stdout.String() != test.stdout || status != test.status || signal != test.signal {
				t.Fatalf("stdout %q status %d signal %d, want %q %d %d", stdout.String(), status, signal, test.stdout, test.status, test.signal)
			}
		})
	}
}
