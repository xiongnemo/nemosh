package runtime

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_privatePipelineScopeCancelsNestedBackgroundBeforeReturning(t *testing.T) {
	// Given
	nestedStarted := make(chan struct{})
	nestedCanceled := make(chan struct{})
	foregroundStarted := make(chan struct{})
	forceRelease := make(chan struct{})
	registry := applets.NewRegistry(
		interruptApplet{name: "nested", run: func(ctx context.Context, _ io.Writer) error {
			close(nestedStarted)
			select {
			case <-ctx.Done():
				close(nestedCanceled)
				return ctx.Err()
			case <-forceRelease:
				return nil
			}
		}},
		interruptApplet{name: "owner", run: func(ctx context.Context, _ io.Writer) error {
			close(foregroundStarted)
			<-ctx.Done()
			return ctx.Err()
		}},
		interruptApplet{name: "peer", run: func(ctx context.Context, _ io.Writer) error {
			<-ctx.Done()
			return ctx.Err()
		}},
	)
	rt := New(registry, Streams{})
	ctx, cancel := InterruptContext(context.Background())
	t.Cleanup(cancel)
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "{ nested & owner; } | peer\n") }()
	awaitOwnerSignal(t, nestedStarted, "nested background did not start")
	awaitOwnerSignal(t, foregroundStarted, "foreground stage did not start")

	// When
	cancel()

	// Then
	select {
	case <-nestedCanceled:
	case <-time.After(2 * time.Second):
		close(forceRelease)
		<-done
		t.Fatal("private pipeline scope did not cancel nested background")
	}
	select {
	case status := <-done:
		if status != 130 {
			t.Fatalf("status = %d, want 130", status)
		}
	case <-time.After(2 * time.Second):
		close(forceRelease)
		t.Fatal("private pipeline teardown did not join nested background")
	}
}

func TestRuntime_privateSubshellScopeCancelsNestedBackgroundBeforeReturning(t *testing.T) {
	// Given
	nestedStarted := make(chan struct{})
	nestedCanceled := make(chan struct{})
	ownerStarted := make(chan struct{})
	registry := applets.NewRegistry(
		interruptApplet{name: "nested", run: func(ctx context.Context, _ io.Writer) error {
			close(nestedStarted)
			<-ctx.Done()
			close(nestedCanceled)
			return ctx.Err()
		}},
		interruptApplet{name: "owner", run: func(ctx context.Context, _ io.Writer) error {
			close(ownerStarted)
			<-ctx.Done()
			return ctx.Err()
		}},
	)
	rt := New(registry, Streams{})
	ctx, cancel := InterruptContext(context.Background())
	t.Cleanup(cancel)
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "(nested & owner)\n") }()
	awaitOwnerSignal(t, nestedStarted, "nested subshell job did not start")
	awaitOwnerSignal(t, ownerStarted, "subshell owner did not start")

	// When
	cancel()

	// Then
	awaitOwnerSignal(t, nestedCanceled, "subshell scope did not cancel nested background")
	select {
	case status := <-done:
		if status != 130 {
			t.Fatalf("status = %d, want 130", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subshell teardown did not join nested background")
	}
}

func TestRuntime_privateSubstitutionScopeCancelsNestedBackgroundBeforeReturning(t *testing.T) {
	// Given
	nestedStarted := make(chan struct{})
	nestedCanceled := make(chan struct{})
	ownerStarted := make(chan struct{})
	registry := applets.NewRegistry(
		interruptApplet{name: "nested", run: func(ctx context.Context, _ io.Writer) error {
			close(nestedStarted)
			<-ctx.Done()
			close(nestedCanceled)
			return ctx.Err()
		}},
		interruptApplet{name: "owner", run: func(ctx context.Context, _ io.Writer) error {
			close(ownerStarted)
			<-ctx.Done()
			return ctx.Err()
		}},
	)
	rt := New(registry, Streams{})
	ctx, cancel := InterruptContext(context.Background())
	t.Cleanup(cancel)
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "echo $(nested & owner)\n") }()
	awaitOwnerSignal(t, nestedStarted, "nested substitution job did not start")
	awaitOwnerSignal(t, ownerStarted, "substitution owner did not start")

	// When
	cancel()

	// Then
	awaitOwnerSignal(t, nestedCanceled, "substitution scope did not cancel nested background")
	select {
	case status := <-done:
		if status != 130 {
			t.Fatalf("status = %d, want 130", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("substitution teardown did not join nested background")
	}
}

func awaitOwnerSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}
