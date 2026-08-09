package runtime

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Leak coverage is deliberately about counts rather than about time.
//
// A wall-clock budget on a shared CI runner flaps -- measured on an idle
// developer machine, five consecutive startups took 43, 45, 42, 26 and 43
// milliseconds, a 42% spread with nothing else running -- and a check that flaps
// gets ignored, which is worse than not having one. A goroutine still running
// after the shell said it was done is a fact rather than a measurement, and it
// stays a fact on a slow machine.
//
// The shapes below are the ones with somewhere to leak: a pipeline starts a
// goroutine per stage, a background job is held by a supervisor, a command
// substitution builds a child runtime with its own descriptor table, and a trap
// runs outside the ordinary flow.

// settledGoroutines waits for the scheduler to quiesce and reports the count.
//
// A goroutine on its way out is not a leak, and Go offers no way to wait for
// that directly, so this polls until the number stops moving. Taking the settled
// reading rather than an immediate one is what keeps the assertion from racing
// the scheduler on a loaded machine.
func settledGoroutines() int {
	previous := -1
	for range 50 {
		goruntime.GC()
		current := goruntime.NumGoroutine()
		if current == previous {
			return current
		}
		previous = current
		time.Sleep(20 * time.Millisecond)
	}
	return previous
}

func runLeakScript(t *testing.T, script string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("script %q exited %d, stderr = %q", script, status, stderr.String())
	}
}

func TestGoroutines_returnToBaseline_afterRepeatedScripts(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
	}{
		{name: "a pipeline", script: "echo one | cat | cat\n"},
		{name: "a long pipeline", script: "echo x | cat | cat | cat | cat | cat\n"},
		{name: "command substitution", script: "x=$(echo inner)\n"},
		{name: "nested substitution", script: "x=$(echo $(echo $(echo deep)))\n"},
		{name: "a background job waited for", script: "true &\nwait\n"},
		{name: "several background jobs", script: "true &\ntrue &\ntrue &\nwait\n"},
		{name: "a background job never waited for", script: "true &\n"},
		{name: "a subshell", script: "(echo inner)\n"},
		{name: "a trap that fires", script: "trap 'echo bye' EXIT\ntrue\n"},
		{name: "a loop with a pipeline inside", script: "for i in 1 2 3; do echo $i | cat; done\n"},
		{name: "a here-document", script: "cat <<EOF\nbody\nEOF\n"},
		{name: "a failing pipeline", script: "false | cat\ntrue\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: one run first, so anything built once and kept -- a lazily
			// populated table, a package-level worker -- already exists and is
			// not counted as growth.
			runLeakScript(t, test.script)
			baseline := settledGoroutines()

			// When
			const repetitions = 25
			for range repetitions {
				runLeakScript(t, test.script)
			}

			// Then: a leak of even one goroutine per run shows up as 25.
			after := settledGoroutines()
			if after > baseline+2 {
				t.Fatalf("goroutines went from %d to %d over %d runs of %q",
					baseline, after, repetitions, test.script)
			}
		})
	}
}

// Tearing a scope down must reclaim its jobs even while they are still running.
//
// This is the counterpart to the tests above, which only prove that jobs which
// finish on their own are reclaimed. `cancelAndDrain` is what every nested scope
// -- a subshell, a pipeline stage, a command substitution -- calls on the way
// out, so a job it fails to stop is one that outlives the construct that owns
// it.
//
// Note what is deliberately not asserted here: cancelling the context passed to
// RunScript does *not* stop a background job. A job runs under the shell's own
// scope, not the caller's, which is what makes Ctrl-C leave background work
// alone, as it does in busybox.
func TestGoroutines_returnToBaseline_whenAScopeIsTornDownWhileJobsRun(t *testing.T) {
	// Given
	baseline := settledGoroutines()

	// When: jobs that would run for half a minute are abandoned by their scope
	const repetitions = 20
	for range repetitions {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{
			Stdin:  strings.NewReader(""),
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if status := rt.RunScript(context.Background(), "sleep 30 &\n"); status != 0 {
			t.Fatalf("starting a background job exited %d, stderr = %q", status, stderr.String())
		}
		rt.jobScope.cancelAndDrain()
	}

	// Then
	after := settledGoroutines()
	if after > baseline+2 {
		t.Fatalf("goroutines went from %d to %d over %d torn-down scopes", baseline, after, repetitions)
	}
}

// One long session rather than many short ones. A leak that is scoped to the
// runtime rather than to a script would be invisible above -- every iteration
// there gets a fresh Runtime, so anything held by the old one becomes garbage --
// and this is the shape a real interactive session has.
func TestGoroutines_returnToBaseline_withinOneLongSession(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	rt.RunScript(context.Background(), "echo warm | cat\n")
	baseline := settledGoroutines()

	// When
	const iterations = 200
	for index := range iterations {
		script := fmt.Sprintf("x=$(echo %d)\necho $x | cat\ntrue &\nwait\n", index)
		if status := rt.RunScript(context.Background(), script); status != 0 {
			t.Fatalf("iteration %d exited %d, stderr = %q", index, status, stderr.String())
		}
	}

	// Then
	after := settledGoroutines()
	if after > baseline+2 {
		t.Fatalf("goroutines went from %d to %d over %d iterations in one session", baseline, after, iterations)
	}
}

// Descriptors are the other thing with somewhere to leak, and a shell that loses
// one per redirect runs out during a long session rather than at once. Two
// hundred redirects is well past any process-wide handle allowance for a leak of
// one per operation, so exhaustion is what a leak looks like from here.
func TestDescriptors_surviveRepeatedRedirects(t *testing.T) {
	// Given: a forward-slash path, because a backslash is an escape to the
	// parser and `C:\Users\...` would arrive as the drive-relative `C:Users...`.
	directory := filepath.ToSlash(t.TempDir())
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// When
	script := new(strings.Builder)
	for index := range 200 {
		fmt.Fprintf(script, "echo %d > %s/out\ncat < %s/out > /dev/null\n", index, directory, directory)
	}
	status := rt.RunScript(context.Background(), script.String())

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr.String())
	}
}
