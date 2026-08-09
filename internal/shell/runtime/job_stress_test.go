package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A long session is where a job table that only grows becomes visible. These
// run one Runtime for many commands rather than one per command, because a
// fresh Runtime per iteration hides exactly the defect they are looking for.

func newStressRuntime(t *testing.T) (Runtime, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: stderr,
	})
	return rt, stdout, stderr
}

// A background job that has finished must not hold its slot forever.
//
// `wait` is what releases a slot, and a script need never call it -- `foo &` on
// its own is ordinary. With maxJobs at 64 that means the sixty-fifth background
// job in a session is refused and every one after it too, permanently, with the
// shell otherwise healthy. Measured against the reference: busybox runs a
// hundred background jobs and starts the hundred-and-first without complaint.
func TestBackgroundJobs_slotIsFreedWhenTheJobFinishes(t *testing.T) {
	// Given
	rt, _, stderr := newStressRuntime(t)

	// When: fill the limit, let the jobs finish without waiting for them, and
	// fill it again -- three times over, so 128 of the 192 jobs can only start
	// if a finished job gave its slot back.
	//
	// Letting them finish is the assertion, not a convenience. The limit bounds
	// jobs that are still *running*, so a burst that outruns the scheduler is
	// entitled to be refused -- on a two-core runner the loop below can queue 64
	// jobs before any of their goroutines is scheduled at all, which is what
	// made an earlier version of this test fail in CI and pass everywhere else.
	// What must never happen is a slot staying spent after its job is over.
	for round := range 3 {
		for range maxJobs {
			if status := rt.RunScript(context.Background(), "true &\n"); status != 0 {
				t.Fatalf("round %d: starting a background job exited %d, stderr = %q", round, status, stderr.String())
			}
		}
		awaitFinishedJobs(t, rt)
	}

	// Then
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// awaitFinishedJobs blocks until every job in the scope has completed, without
// calling `wait` -- `wait` releases the slots itself, which is the very thing
// under test here.
func awaitFinishedJobs(t *testing.T, rt Runtime) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		pending := 0
		for _, record := range rt.jobScope.snapshot() {
			select {
			case <-record.done:
			default:
				pending++
			}
		}
		if pending == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d background jobs never finished", pending)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The bound itself must survive the fix. Sixty-four jobs that are all still
// running is the case the limit exists for, and a sweep of finished jobs must
// not turn it into no limit at all.
func TestBackgroundJobs_limitStillRefusesWhenTheJobsAreRunning(t *testing.T) {
	// Given
	rt, _, stderr := newStressRuntime(t)

	// When: more simultaneous long-running jobs than the limit allows
	script := new(strings.Builder)
	for range maxJobs + 1 {
		script.WriteString("sleep 30 &\n")
	}
	rt.RunScript(context.Background(), script.String())

	// Then
	if !strings.Contains(stderr.String(), "job limit reached") {
		t.Fatalf("stderr = %q, want the job limit to refuse the %dth running job", stderr.String(), maxJobs+1)
	}

	// Cleanup: the sleeps hold goroutines for half a minute otherwise.
	rt.jobScope.cancelAndDrain()
}

// A finished job stays reportable while there is room for it. Sweeping on every
// registration would be simpler, but `jobs` shows Done entries in busybox and
// `wait %N` addresses a job by number, so the sweep is deliberately something
// that only happens under pressure.
func TestBackgroundJobs_finishedJobRemainsReportableBelowTheLimit(t *testing.T) {
	// Given
	rt, stdout, stderr := newStressRuntime(t)

	// When
	if status := rt.RunScript(context.Background(), "true &\n"); status != 0 {
		t.Fatalf("starting a background job exited %d, stderr = %q", status, stderr.String())
	}
	<-rt.jobScope.snapshot()[0].done
	if status := rt.RunScript(context.Background(), "jobs\n"); status != 0 {
		t.Fatalf("jobs exited %d, stderr = %q", status, stderr.String())
	}

	// Then
	if got, want := stdout.String(), "[1] Done\n"; got != want {
		t.Fatalf("jobs printed %q, want %q", got, want)
	}
}
