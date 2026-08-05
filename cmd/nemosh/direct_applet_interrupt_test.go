package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type directInterruptApplet struct {
	started chan struct{}
	stopped chan struct{}
}

func (a directInterruptApplet) Name() string { return "held" }

func (a directInterruptApplet) Run(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
	close(a.started)
	<-ctx.Done()
	close(a.stopped)
	return ctx.Err()
}

func TestCommand_directAppletInterruptReturns130AfterQuiescence(t *testing.T) {
	for _, args := range [][]string{{directAppletInvocationName("held")}, {"nemosh", "held"}} {
		// Given
		started := make(chan struct{})
		stopped := make(chan struct{})
		signals := make(chan os.Signal, 1)
		cmd := command{stdin: bytes.NewReader(nil), stdout: io.Discard, stderr: io.Discard, interrupts: signals, registry: applets.NewRegistry(directInterruptApplet{started, stopped})}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		done := make(chan error, 1)
		go func() { done <- cmd.run(ctx, args) }()
		awaitDirectSignal(t, started, "direct applet did not start")

		// When
		signals <- os.Interrupt

		// Then
		awaitDirectSignal(t, stopped, "direct applet did not quiesce")
		select {
		case err := <-done:
			if status := interactiveStatus(t, err); status != 130 {
				t.Fatalf("args = %v, status = %d, want 130", args, status)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("args = %v did not return", args)
		}
	}
}

func awaitDirectSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}
