package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

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

	// When: well past maxJobs, none of them waited for, each finishing at once
	const jobs = maxJobs * 3
	for range jobs {
		if status := rt.RunScript(context.Background(), "true &\n"); status != 0 {
			t.Fatalf("starting a background job exited %d, stderr = %q", status, stderr.String())
		}
	}

	// Then
	if strings.Contains(stderr.String(), "job limit reached") {
		t.Fatalf("the job limit was hit after finished jobs should have freed their slots; stderr = %q", stderr.String())
	}

	// And: a further job still starts, which is the failure a session would
	// actually notice.
	stderr.Reset()
	if status := rt.RunScript(context.Background(), "true &\nwait\n"); status != 0 {
		t.Fatalf("a background job after %d earlier ones exited %d, stderr = %q", jobs, status, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
