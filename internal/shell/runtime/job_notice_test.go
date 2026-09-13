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

// **A job that has ended says so at the next prompt.**
//
// Asked for at a terminal: after `kill %1` the prompt came back with nothing to say, and
// the only way to learn the job had gone was to run `jobs`.
//
// bash and busybox both report this before the next prompt, and neither needs `set -b` to
// do it -- `set -b` asks for the report *immediately*, mid-command, which is the part this
// shell has no channel for and still refuses. Reporting at a prompt needs no channel: the
// loop is about to draw one, so it asks first.
//
// On stderr, where busybox puts it and where the launch announcement already goes, so that
// a job ending during `x=$(...)` cannot land in the captured value.
//
// The wording is shared with the `jobs` builtin rather than written twice. They are the
// same statement about the same record, and a notice that drifted from what `jobs` says
// would be worse than no notice.

func TestReportFinishedJobs_namesAJobThatHasEndedAndForgetsIt(t *testing.T) {
	started := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "blocker", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}})
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	run := func(text string) {
		t.Helper()
		script, err := runtime.ParseScript(text)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		rt.RunInteractive(context.Background(), script)
	}

	run("blocker &\n")
	<-started
	stderr.Reset()

	// While it runs there is nothing to say.
	rt.ReportFinishedJobs()
	if stderr.Len() != 0 {
		t.Fatalf("a running job was announced as finished: %q", stderr.String())
	}

	run("kill %1\n")
	// Asked until it has something, because `kill` cancels a context and the applet
	// returns on its own time. The call that reports it is the one that consumes it, so
	// the assertion below still lands on the right one.
	var notice string
	for range 400 {
		stderr.Reset()
		rt.ReportFinishedJobs()
		if notice = stderr.String(); notice != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(notice, "[1]") || !strings.Contains(notice, "Done") {
		t.Fatalf("the notice was %q, want it to name job 1 as done", notice)
	}

	// Said once. A second ask has nothing, and neither has `jobs`.
	stderr.Reset()
	rt.ReportFinishedJobs()
	if stderr.Len() != 0 {
		t.Errorf("the same job was announced twice: %q", stderr.String())
	}
	stdout.Reset()
	run("jobs\n")
	if stdout.Len() != 0 {
		t.Errorf("jobs still lists it after the notice: %q", stdout.String())
	}
}

// TestReportFinishedJobs_saysTheSameThingAsJobs keeps the two wordings from drifting: a
// notice that called it something `jobs` does not would be two accounts of one record.
func TestReportFinishedJobs_saysTheSameThingAsJobs(t *testing.T) {
	registry := applets.NewRegistry(backgroundApplet{name: "failer", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		return applets.ExitStatus(3)
	}})
	shellForJobs := newInteractiveShell(t, registry)
	shellForJobs.run("failer &\n")
	shellForJobs.run("wait\n")

	// `wait` consumes the record, so a second launch is needed to see what `jobs` says.
	shellForJobs.run("failer &\n")
	var listed string
	for range 400 {
		listed = shellForJobs.run("jobs\n")
		if strings.Contains(listed, "Done") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script, err := runtime.ParseScript("failer &\n")
	if err != nil {
		t.Fatal(err)
	}
	rt.RunInteractive(context.Background(), script)
	var notice string
	for range 400 {
		stderr.Reset()
		rt.ReportFinishedJobs()
		if notice = stderr.String(); notice != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if listed == "" || notice == "" {
		t.Fatalf("nothing to compare: jobs said %q and the notice said %q", listed, notice)
	}
	// The state, not the number: these are two shells with their own job counters, and the
	// claim being tested is about the words after the number.
	state := func(line string) string {
		if _, after, found := strings.Cut(line, "] "); found {
			return after
		}
		return line
	}
	if state(listed) != state(notice) {
		t.Errorf("jobs says %q and the notice says %q; they describe the same record and "+
			"should not have two wordings", listed, notice)
	}
	// And the status really did travel, so this is not two empty strings agreeing.
	if !strings.Contains(state(notice), "Done(3)") {
		t.Errorf("the notice said %q, want the applet's status 3 in it", notice)
	}
}
