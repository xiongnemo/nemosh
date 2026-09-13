package runtime_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **`jobs` reports a finished job once, and then it is gone.**
//
// Reported from a real terminal:
//
//	$ kill %1
//	$ jobs
//	[1] Done(1)
//	$ jobs
//	[1] Done(1)
//	$ jobs
//	[1] Done(1)
//
// POSIX 2.9.3 has the shell remove a job from the list once it has reported the status,
// and busybox does: measured, `[1]+ Done sleep 1` appears once and `jobs` afterwards says
// nothing. Reporting it forever is worse than untidy -- the table never shrinks, and
// `jobs` stops describing what is actually running.
//
// Only what was reported is removed, rather than sweeping everything finished, so a job
// that ends between the listing and the sweep is reported next time instead of
// disappearing unannounced.
//
// docs/support-matrix.md already describes the model this belongs to: there is no
// asynchronous notification here -- no `[1]+ Done` before a prompt, the way busybox does
// it -- and completion "is reported when `wait` or `jobs` asks". Asking is what consumes
// it.

// interactiveShell runs lines the way a prompt does, keeping the status between them.
type interactiveShell struct {
	t      *testing.T
	rt     *runtime.Runtime
	stdout *bytes.Buffer
}

func (s *interactiveShell) run(text string) string {
	s.t.Helper()
	script, err := runtime.ParseScript(text)
	if err != nil {
		s.t.Fatalf("parse %q: %v", text, err)
	}
	s.stdout.Reset()
	s.rt.RunInteractive(context.Background(), script)
	return s.stdout.String()
}

func newInteractiveShell(t *testing.T, registry applets.Registry) *interactiveShell {
	t.Helper()
	stdout := &bytes.Buffer{}
	rt := runtime.New(registry, runtime.Streams{Stdout: stdout, Stderr: io.Discard})
	return &interactiveShell{t: t, rt: &rt, stdout: stdout}
}

func TestJobs_reportsAFinishedJobOnceAndThenForgetsIt(t *testing.T) {
	started := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "blocker", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}})
	shell := newInteractiveShell(t, registry)

	shell.run("blocker &\n")
	<-started
	if running := shell.run("jobs\n"); running != "[1] Running\n" {
		t.Fatalf("while running, jobs = %q, want %q", running, "[1] Running\n")
	}

	// `kill` does not claim the record, so `jobs` is the only thing that can consume it --
	// which is the sequence the terminal hit.
	shell.run("kill %1\n")

	// Asked until it answers Done rather than once after a pause: `kill` cancels the
	// context and the applet returns on its own time, so a single ask can catch it still
	// Running -- which it did, under -race. The ask that sees Done *is* the report, so the
	// assertion below still lands on the right one, and nothing here depends on how fast
	// the machine is.
	var first string
	for range 400 {
		first = shell.run("jobs\n")
		if strings.Contains(first, "Done") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(first, "Done") {
		t.Fatalf("the killed job never reported Done; last answer was %q", first)
	}
	if second := shell.run("jobs\n"); second != "" {
		t.Errorf("jobs answered %q and then %q; a job whose status has been reported is "+
			"removed from the list, so the second ask has nothing left to say", first, second)
	}
}

// TestJobs_doesNotForgetAJobStillRunning is the other half, and the one that would make
// this fix worse than the bug: a running job listed by `jobs` has not reported anything
// final, so it stays and stays addressable.
func TestJobs_doesNotForgetAJobStillRunning(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "worker", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		<-release
		return nil
	}})
	shell := newInteractiveShell(t, registry)

	shell.run("worker &\n")
	<-started
	for attempt := range 3 {
		if listed := shell.run("jobs\n"); listed != "[1] Running\n" {
			t.Fatalf("ask %d: jobs = %q, want the job still listed", attempt+1, listed)
		}
	}
	// And still addressable afterwards, which is what `jobs` forgetting it would break.
	close(release)
	shell.run("wait %1\n")
}
