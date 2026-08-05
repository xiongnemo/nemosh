package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunInteractive_interruptsReadBeforeFreshInputAndPreservesFreshLine(t *testing.T) {
	// Given
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	var closeWriter sync.Once
	closeInput := func() { closeWriter.Do(func() { _ = writer.Close() }) }
	t.Cleanup(func() {
		closeInput()
		_ = reader.Close()
	})
	trackedInput := newTrackingReader(reader)
	signals := make(chan os.Signal, 1)
	stdout := newLiveOutput()
	stderr := newLiveOutput()
	done := make(chan error, 1)
	cmd := command{stdin: trackedInput, stdout: stdout, stderr: stderr, interrupts: signals}
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-i"}) }()
	if _, err := fmt.Fprintln(writer, "PS1='P> '"); err != nil {
		t.Fatalf("write PS1: %v", err)
	}
	awaitTextCount(t, stderr, "P> ", 1)
	base := awaitOutstandingRead(t, trackedInput, 0)
	if _, err := fmt.Fprintln(writer, "read value"); err != nil {
		t.Fatalf("write read command: %v", err)
	}
	awaitOutstandingRead(t, trackedInput, base+uint64(len("read value\n")))

	// When
	signals <- os.Interrupt

	// Then
	if !awaitTextCountBounded(stderr, "P> ", 2, time.Second) {
		closeInput()
		awaitError(t, done, "interrupted read cleanup")
		t.Fatalf("stderr = %q, want replacement prompt before fresh input", stderr.String())
	}
	if _, err := fmt.Fprintln(writer, "echo fresh"); err != nil {
		t.Fatalf("write fresh command: %v", err)
	}
	if _, err := fmt.Fprintln(writer, "exit 0"); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	closeInput()
	if err := awaitError(t, done, "interactive read recovery"); err != nil {
		t.Fatalf("interactive run: %v", err)
	}
	if got := stdout.String(); got != "fresh\n" {
		t.Fatalf("stdout = %q, want %q", got, "fresh\n")
	}
	if strings.Contains(stderr.String(), "read:") {
		t.Fatalf("stderr = %q, want no read cancellation diagnostic", stderr.String())
	}
}

func awaitTextCountBounded(output *liveOutput, text string, count int, limit time.Duration) bool {
	timer := time.NewTimer(limit)
	defer timer.Stop()
	for strings.Count(output.String(), text) < count {
		select {
		case <-output.updated:
		case <-timer.C:
			return false
		}
	}
	return true
}
