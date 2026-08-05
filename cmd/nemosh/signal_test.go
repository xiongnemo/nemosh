package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInterruptController_firstInterruptDoesNotPoisonNextExecution(t *testing.T) {
	// Given
	controller := &interruptController{}
	first, clearFirst := controller.context(context.Background())

	// When
	controller.interrupt()
	clearFirst()
	second, clearSecond := controller.context(context.Background())
	defer clearSecond()

	// Then
	if first.Err() == nil {
		t.Fatal("first execution context remained live")
	}
	if second.Err() != nil {
		t.Fatalf("second execution context = %v, want live", second.Err())
	}
}

func TestInterruptController_staleFinishDoesNotClearNewExecution(t *testing.T) {
	// Given
	controller := &interruptController{}
	_, finishFirst := controller.context(context.Background())
	second, finishSecond := controller.context(context.Background())
	defer finishSecond()
	finishFirst()

	// When
	controller.interrupt()

	// Then
	if second.Err() == nil {
		t.Fatal("stale first finish cleared the second execution interrupt")
	}
}

func TestInterruptController_finishReleasesExecutionContext(t *testing.T) {
	// Given
	controller := &interruptController{}
	ctx, finish := controller.context(context.Background())

	// When
	finish()

	// Then
	if ctx.Err() == nil {
		t.Fatal("finished execution context remained live")
	}
}

type readyWriter struct {
	buffer bytes.Buffer
	ready  chan struct{}
}

func (w *readyWriter) Write(data []byte) (int, error) {
	count, err := w.buffer.Write(data)
	if strings.Contains(w.buffer.String(), "READY\n") {
		select {
		case <-w.ready:
		default:
			close(w.ready)
		}
	}
	return count, err
}

func TestCommand_interruptCancelsRealExternalAndReturns130(t *testing.T) {
	// Given
	signals := make(chan os.Signal, 1)
	stdout := &readyWriter{ready: make(chan struct{})}
	cmd := command{stdin: bytes.NewReader(nil), stdout: stdout, stderr: io.Discard, interrupts: signals}
	executable := filepath.ToSlash(os.Args[0])
	script := fmt.Sprintf("trap 'echo int:$?' INT\n%s -test.run=TestSignalHelperProcess -- signal-helper\n", executable)
	done := make(chan error, 1)
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-c", script}) }()
	awaitSignal(t, stdout.ready, "external helper readiness")

	// When
	signals <- os.Interrupt
	err := awaitError(t, done, "external interrupt completion")

	// Then
	if status := interactiveStatus(t, err); status != 130 {
		t.Fatalf("status = %d, stdout = %q, want 130", status, stdout.buffer.String())
	}
	if stdout.buffer.String() != "READY\nint:130\n" {
		t.Fatalf("stdout = %q, want quiesced helper then INT trap", stdout.buffer.String())
	}
}

func TestSignalHelperProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "signal-helper" {
		return
	}
	fmt.Println("READY")
	select {}
}
