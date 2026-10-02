package applets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
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
