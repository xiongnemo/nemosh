package applets

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// **A read that is already blocked ends when the command is interrupted.**
//
// Reported from a real terminal: Ctrl-C inside `bc` did nothing until another key was
// pressed, and that key was swallowed noticing it -- so it took two presses, and after the
// first the shell had already left bc without drawing anything.
//
// readWithContext checked ctx.Err() before starting a read and never again, which is no
// help to an applet sitting in one: a console read returns when a line arrives and not
// before. Measured on a real console first -- a read blocked for 500ms, CancelIoEx answered
// success, and the read returned at once -- because the alternative, waiting on the console
// handle, does not work here: ENABLE_LINE_INPUT signals the handle on every keystroke while
// ReadFile keeps waiting for Enter.
//
// A pipe rather than a console, so this runs where there is no terminal. The handle type
// differs; what is being tested -- that a blocked read is ended rather than waited on -- is
// the same call, and token_pipeline.go already cancels pipe I/O this way.

func TestReadWithContext_endsAReadThatHasAlreadyBlocked(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		buffer := make([]byte, 16)
		_, readErr := readWithContext(ctx, reader, buffer)
		done <- readErr
	}()

	// The control: nothing has been written, so the read must still be waiting. Without
	// this the test would pass against a read that had already given up on its own.
	select {
	case readErr := <-done:
		t.Fatalf("the read returned %v on its own, so this cannot measure cancellation", readErr)
	case <-time.After(250 * time.Millisecond):
	}

	cancel()
	select {
	case readErr := <-done:
		if !errors.Is(readErr, context.Canceled) {
			t.Fatalf("the cancelled read answered %v, want context.Canceled -- an aborted read "+
				"reports EOF, so the context has to be what decides", readErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the read is still blocked after being cancelled, so Ctrl-C still needs a keystroke")
	}
}

// TestReadWithContext_doesNotStealDataFromAReadThatSucceeded keeps the cancellation from
// costing input: bytes that arrived belong to the applet, and the next read is where the
// cancellation is reported.
func TestReadWithContext_doesNotStealDataFromAReadThatSucceeded(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close(); writer.Close() })
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buffer := make([]byte, 16)
	read, err := readWithContext(ctx, reader, buffer)
	if err != nil || string(buffer[:read]) != "hello" {
		t.Fatalf("read %q with err %v, want the five bytes that were written", buffer[:read], err)
	}
}
