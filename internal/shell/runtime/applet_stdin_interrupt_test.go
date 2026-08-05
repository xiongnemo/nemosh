package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

var errPlainAppletRead = errors.New("plain Read called instead of ReadContext")

type interruptibleAppletInput struct {
	contextReadStarted chan struct{}
	plainReadCalled    chan struct{}
}

func (i interruptibleAppletInput) Read([]byte) (int, error) {
	select {
	case i.plainReadCalled <- struct{}{}:
	default:
	}
	return 0, errPlainAppletRead
}

func (i interruptibleAppletInput) ReadContext(ctx context.Context, _ []byte) (int, error) {
	close(i.contextReadStarted)
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestRuntime_catReturns130WhenInterruptedDuringContextualStdinRead(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{name: "plain stdin", script: "cat\n"},
		{name: "device stdin", script: "cat /dev/stdin\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			input := interruptibleAppletInput{
				contextReadStarted: make(chan struct{}),
				plainReadCalled:    make(chan struct{}, 1),
			}
			var stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: input, Stderr: &stderr})
			ctx, interrupt := runtime.InterruptContext(context.Background())
			t.Cleanup(interrupt)
			done := make(chan int, 1)
			go func() { done <- rt.RunScript(ctx, test.script) }()

			select {
			case <-input.contextReadStarted:
			case <-input.plainReadCalled:
				select {
				case status := <-done:
					t.Fatalf("plain Read used instead of ReadContext: status=%d stderr=%q", status, stderr.String())
				case <-time.After(time.Second):
					t.Fatal("plain Read reported but cat did not finish")
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for applet stdin read")
			}

			// When
			interrupt()

			// Then
			select {
			case status := <-done:
				if status != 130 {
					t.Fatalf("status = %d, want 130; stderr=%q", status, stderr.String())
				}
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for interrupted cat")
			}
		})
	}
}
