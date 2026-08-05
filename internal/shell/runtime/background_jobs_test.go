package runtime_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

type backgroundApplet struct {
	name string
	run  func(context.Context, []string, io.Reader, io.Writer, io.Writer) error
}

func (a backgroundApplet) Name() string { return a.name }

func (a backgroundApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return a.run(ctx, args, stdin, stdout, stderr)
}

func TestRuntime_backgroundLaunchContinuesImmediately_whenWorkerIsBlocked(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	continued := make(chan struct{})
	registry := applets.NewRegistry(
		backgroundApplet{name: "block", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		backgroundApplet{name: "continued", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(continued)
			return nil
		}},
	)
	rt := runtime.New(registry, runtime.Streams{})
	result := make(chan int, 1)
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)

	// When
	go func() { result <- rt.RunScript(context.Background(), "block & continued\n") }()

	// Then
	select {
	case <-continued:
	case <-time.After(2 * time.Second):
		t.Fatal("parent did not continue while background worker remained blocked")
	}
	select {
	case status := <-result:
		if status != 0 {
			t.Fatalf("parent status = %d, want 0", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parent did not return immediately after background launch")
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background worker did not start")
	}
}

func TestRuntime_jobsObservesRunningAndWaitReturnsCachedStatus_whenWorkerCompletes(t *testing.T) {
	// Given
	started := make(chan struct{})
	release := make(chan struct{})
	registry := applets.NewRegistry(backgroundApplet{name: "block-false", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		close(started)
		select {
		case <-release:
			return applets.ErrExitFalse
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	var stdout, stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	launch := make(chan int, 1)
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(closeRelease)

	// When
	go func() { launch <- rt.RunScript(context.Background(), "block-false &\n") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background worker did not start")
	}
	jobsStatus := rt.RunScript(context.Background(), "jobs\n")
	closeRelease()
	select {
	case status := <-launch:
		if status != 0 {
			t.Fatalf("launch status = %d, want 0", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("background launch did not return")
	}
	waitStatus := rt.RunScript(context.Background(), "wait %1\n")

	// Then
	if jobsStatus != 0 || stdout.String() != "[1] Running\n" {
		t.Fatalf("jobs status = %d, stdout = %q", jobsStatus, stdout.String())
	}
	if waitStatus != 1 || stderr.String() != "" {
		t.Fatalf("wait status = %d, stderr = %q", waitStatus, stderr.String())
	}
}
