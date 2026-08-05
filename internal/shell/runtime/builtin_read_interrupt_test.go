package runtime

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type interruptibleReadInput struct {
	started chan struct{}
}

func (i interruptibleReadInput) Read([]byte) (int, error) {
	panic("Read called instead of ReadContext")
}

func (i interruptibleReadInput) ReadContext(ctx context.Context, _ []byte) (int, error) {
	close(i.started)
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestRuntime_readReturns130SilentlyWhenShellInterrupted(t *testing.T) {
	// Given
	started := make(chan struct{})
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: interruptibleReadInput{started: started}, Stderr: &stderr})
	ctx, interrupt := InterruptContext(context.Background())
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "read value\n") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for builtin read")
	}

	// When
	interrupt()
	var status int
	select {
	case status = <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for interrupted builtin read")
	}

	// Then
	if status != 130 {
		t.Fatalf("status = %d, want 130", status)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want no read cancellation diagnostic", stderr.String())
	}
}
