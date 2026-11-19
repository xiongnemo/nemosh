package applets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/cancelio"
)

// tee -i goes on through an interrupt, as busybox's ignores SIGINT: what comes after ^C still
// reaches its FILEs, until its input ends. Any other end of its context still ends it, and
// without -i an interrupt does.
func TestTee_goesOnThroughAnInterruptWithI(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	reader, writer := io.Pipe()
	var stdout bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- newTeeApplet().Run(ctx, []string{"-i"}, reader, &stdout, io.Discard) }()
	if _, err := writer.Write([]byte("before ")); err != nil {
		t.Fatal(err)
	}
	cancel(ErrInterrupt)
	if _, err := writer.Write([]byte("after")); err != nil {
		t.Fatalf("tee -i stopped reading at the interrupt: %v", err)
	}
	writer.Close()
	if err := <-done; err != nil || stdout.String() != "before after" {
		t.Errorf("tee -i across an interrupt: %q, %v; want before after", stdout.String(), err)
	}

	for _, test := range []struct {
		args  []string
		cause error
	}{
		{nil, ErrInterrupt},
		{[]string{"-i"}, errors.New("terminated")},
	} {
		// Ended before tee starts: a read on an io.Pipe that has begun is not one the context
		// can end, so cancelling after the start raced the first read, and lost on Linux.
		ctx, cancel := context.WithCancelCause(context.Background())
		reader, writer := io.Pipe()
		cancel(test.cause)
		done := make(chan error, 1)
		go func() { done <- newTeeApplet().Run(ctx, test.args, reader, io.Discard, io.Discard) }()
		if err := <-done; err == nil {
			t.Errorf("tee %q ended by %v: nil, want it to stop", test.args, test.cause)
		}
		writer.Close()
	}
}

// Pinned, as the shell runs an applet, tee -i still goes on: the pin cancels the calls of a
// command an interrupt ended, and tee -i is one the interrupt did not end. Its read of a pipe
// waits five of the pin's intervals for what comes after the ^C.
func TestTee_goesOnThroughAnInterruptWhilePinned(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	stdout := &copiedOutput{copied: make(chan struct{}, 8)}
	done := make(chan error, 1)
	go func() {
		defer cancelio.Pin(ctx)()
		done <- newTeeApplet().Run(ctx, []string{"-i"}, reader, stdout, io.Discard)
	}()
	writer.Write([]byte("before "))
	// The interrupt comes once tee has copied that and waits to read on: a write to a pipe
	// returns at once, and an interrupt before tee had started ended it in its options.
	select {
	case <-stdout.copied:
	case <-time.After(5 * time.Second):
		t.Fatal("tee -i never copied what came before the interrupt")
	}
	cancel(ErrInterrupt)
	time.Sleep(50 * time.Millisecond)
	writer.Write([]byte("after"))
	writer.Close()
	select {
	case err := <-done:
		if got := stdout.String(); err != nil || got != "before after" {
			t.Fatalf("pinned tee -i across an interrupt: %q, %v; want before after", got, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pinned tee -i did not end with its input")
	}
}

// copiedOutput is a writer a test can read while it is written, which says each write.
type copiedOutput struct {
	mu     sync.Mutex
	text   bytes.Buffer
	copied chan struct{}
}

func (o *copiedOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.text.Write(data)
	select {
	case o.copied <- struct{}{}:
	default:
	}
	return len(data), nil
}

func (o *copiedOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.text.String()
}
