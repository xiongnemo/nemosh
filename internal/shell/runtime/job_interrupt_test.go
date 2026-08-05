package runtime

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type interruptApplet struct {
	name string
	run  func(context.Context, io.Writer) error
}

func (a interruptApplet) Name() string { return a.name }

func (a interruptApplet) Run(ctx context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
	return a.run(ctx, stdout)
}

func TestRuntime_waitCancellationReturns130RetainsRecordAndRetryStatus(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(interruptApplet{name: "held", run: func(_ context.Context, _ io.Writer) error {
		close(started)
		<-release
		return applets.ExitStatus(7)
	}})
	rt := New(registry, Streams{})
	if status := rt.RunScript(context.Background(), "held &\n"); status != 0 {
		t.Fatalf("launch status = %d, want 0", status)
	}
	<-started
	ctx, cancel := InterruptContext(context.Background())
	cancel()

	// When
	status := rt.RunScript(ctx, "wait %1\n")

	// Then
	if status != 130 || len(rt.jobScope.snapshot()) != 1 || rt.jobScope.snapshot()[0].claimed {
		t.Fatalf("status = %d, records = %#v, want 130 and one unclaimed record", status, rt.jobScope.snapshot())
	}
	close(release)
	if retry := rt.RunScript(context.Background(), "wait %1\n"); retry != 7 {
		t.Fatalf("retry status = %d, want 7", retry)
	}
}

func TestRuntime_foregroundCancellationQuiescesThenRunsINTTrapOnce(t *testing.T) {
	// Given
	started := make(chan struct{})
	quiesced := make(chan struct{})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(interruptApplet{name: "cooperative", run: func(ctx context.Context, _ io.Writer) error {
		close(started)
		<-ctx.Done()
		close(quiesced)
		return ctx.Err()
	}})
	rt := New(registry, Streams{Stdout: &stdout, Stderr: &stderr})
	ctx, cancel := InterruptContext(context.Background())
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "trap 'echo int:$?' INT\ncooperative\necho unreachable\n") }()
	<-started

	// When
	cancel()
	status := <-done

	// Then
	select {
	case <-quiesced:
	default:
		t.Fatal("runtime returned before foreground applet quiesced")
	}
	if status != 130 || stdout.String() != "int:130\n" {
		t.Fatalf("status = %d, stdout = %q, want 130 and one INT trap", status, stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want no context-canceled diagnostic", stderr.String())
	}
}

func TestRuntime_exitTrapCanWaitBeforeRootScopeSeals(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false &\ntrap 'jobs\nwait %1\necho exit:$?' EXIT\n")
	rt.CloseBatch(0)

	// Then
	if status != 0 || !strings.HasSuffix(stdout.String(), "exit:1\n") {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
	if _, err := rt.jobScope.register(); err == nil {
		t.Fatal("root job scope accepted launch after EXIT trap")
	}
}

func TestRuntime_backgroundClearsINTAndEXITTraps(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo parent-int' INT\ntrap 'echo parent-exit' EXIT\n{ trap 'echo child-int' INT; trap 'echo child-exit' EXIT; } &\nwait %1\n")

	// Then
	if status != 0 || stdout.String() != "parent-exit\n" {
		t.Fatalf("status = %d, stdout = %q, want only parent EXIT trap", status, stdout.String())
	}
}

func TestRuntime_contextInsensitiveLoopStopsAtCommandBoundary(t *testing.T) {
	// Given
	ctx, cancel := InterruptContext(context.Background())
	calls := 0
	registry := applets.NewRegistry(interruptApplet{name: "cancel-once", run: func(_ context.Context, _ io.Writer) error {
		calls++
		cancel()
		return nil
	}})
	rt := New(registry, Streams{})

	// When
	status := rt.RunScript(ctx, "while true\ndo\ncancel-once\ndone\n")

	// Then
	if status != 130 || calls != 1 {
		t.Fatalf("status = %d, calls = %d, want 130 and one command", status, calls)
	}
}

func TestRuntime_interactiveInterruptDoesNotPoisonNextEntry(t *testing.T) {
	// Given
	started := make(chan struct{})
	registry := applets.NewRegistry(interruptApplet{name: "held", run: func(ctx context.Context, _ io.Writer) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}})
	var stdout bytes.Buffer
	rt := New(registry, Streams{Stdout: &stdout})
	first, _ := ParseScript("held\n")
	second, _ := ParseScript("echo alive\n")
	ctx, cancel := InterruptContext(context.Background())
	done := make(chan InteractiveResult, 1)
	go func() { done <- rt.RunInteractive(ctx, first) }()
	<-started

	// When
	cancel()
	interrupted := <-done
	recovered := rt.RunInteractive(context.Background(), second)

	// Then
	if interrupted.Status != 130 || interrupted.Exited || recovered.Status != 0 || stdout.String() != "alive\n" {
		t.Fatalf("interrupted = %+v, recovered = %+v, stdout = %q", interrupted, recovered, stdout.String())
	}
}

func TestRuntime_closeDoesNotWaitForOrKillLiveRootJob(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(interruptApplet{name: "held", run: func(_ context.Context, _ io.Writer) error {
		close(started)
		<-release
		return nil
	}})
	rt := New(registry, Streams{})
	if status := rt.RunScript(context.Background(), "held &\n"); status != 0 {
		t.Fatalf("launch status = %d", status)
	}
	<-started
	closed := make(chan struct{})

	// When
	go func() {
		rt.CloseBatch(0)
		close(closed)
	}()
	<-closed

	// Then
	if len(rt.jobScope.snapshot()) != 1 {
		t.Fatalf("jobs = %#v, want retained live job", rt.jobScope.snapshot())
	}
	close(release)
	if status := rt.wait(context.Background(), []string{"%1"}); status != 0 {
		t.Fatalf("post-close wait status = %d, want 0", status)
	}
}
