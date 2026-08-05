package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type contextBlockingReader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newContextBlockingReader() *contextBlockingReader {
	return &contextBlockingReader{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (r *contextBlockingReader) Read([]byte) (int, error) {
	r.notifyStarted()
	<-r.release
	return 0, io.EOF
}

func (r *contextBlockingReader) ReadContext(ctx context.Context, _ []byte) (int, error) {
	r.notifyStarted()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-r.release:
		return 0, io.EOF
	}
}

func (r *contextBlockingReader) notifyStarted() {
	select {
	case r.started <- struct{}{}:
	default:
	}
}

func (r *contextBlockingReader) unblock() {
	r.once.Do(func() { close(r.release) })
}

func TestRunInteractive_returnsWhenParentContextIsCanceledDuringBlockedInput(t *testing.T) {
	// Given
	reader := newContextBlockingReader()
	t.Cleanup(reader.unblock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	cmd := command{stdin: reader, stdout: io.Discard, stderr: io.Discard}
	go func() { done <- cmd.run(ctx, []string{"nemosh", "-i"}) }()
	awaitSignal(t, reader.started, "interactive stdin read")

	// When
	cancel()

	// Then
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("interactive run error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		reader.unblock()
		awaitError(t, done, "interactive cancellation cleanup")
		t.Fatal("interactive run did not return after parent cancellation")
	}
}

func awaitSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func awaitError(t *testing.T, result <-chan error, name string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
		return nil
	}
}
