package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

type trackingReader struct {
	reader   io.Reader
	mu       sync.Mutex
	started  uint64
	returned uint64
	updated  chan struct{}
}

func newTrackingReader(reader io.Reader) *trackingReader {
	return &trackingReader{reader: reader, updated: make(chan struct{}, 1)}
}

func (r *trackingReader) Read(buffer []byte) (int, error) {
	r.mu.Lock()
	r.started++
	r.mu.Unlock()
	r.notify()
	count, err := r.reader.Read(buffer)
	r.mu.Lock()
	r.returned++
	r.mu.Unlock()
	r.notify()
	return count, err
}

func (r *trackingReader) InteractiveInputFile() *os.File {
	file, _ := r.reader.(*os.File)
	return file
}

func (r *trackingReader) notify() {
	select {
	case r.updated <- struct{}{}:
	default:
	}
}

func awaitOutstandingRead(t *testing.T, reader *trackingReader, minimum uint64) uint64 {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		reader.mu.Lock()
		started, returned := reader.started, reader.returned
		reader.mu.Unlock()
		if started >= minimum && started == returned+1 {
			return started
		}
		select {
		case <-reader.updated:
		case <-timer.C:
			t.Fatalf("reads = (%d started, %d returned), want one outstanding at or after %d", started, returned, minimum)
		}
	}
}

func TestRunInteractive_idleInterruptClearsContinuationAndReprompts(t *testing.T) {
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
	signals := make(chan os.Signal, 1)
	stdout := newLiveOutput()
	stderr := newLiveOutput()
	done := make(chan error, 1)
	trackedInput := newTrackingReader(reader)
	cmd := command{stdin: trackedInput, stdout: stdout, stderr: stderr, interrupts: signals}
	go func() { done <- cmd.run(context.Background(), []string{"nemosh", "-i"}) }()
	ps1Line := "PS1='P> '\n"
	if _, err := writer.WriteString(ps1Line); err != nil {
		t.Fatalf("write PS1: %v", err)
	}
	awaitTextCount(t, stderr, "P> ", 1)
	ps2Line := "PS2='C> '\n"
	if _, err := writer.WriteString(ps2Line); err != nil {
		t.Fatalf("write PS2: %v", err)
	}
	awaitTextCount(t, stderr, "P> ", 2)
	incompleteLine := "if true\n"
	if _, err := writer.WriteString(incompleteLine); err != nil {
		t.Fatalf("write incomplete command: %v", err)
	}
	awaitTextCount(t, stderr, "C> ", 1)
	base := awaitOutstandingRead(t, trackedInput, 0)
	partialLine := "echo stale"
	if _, err := writer.WriteString(partialLine); err != nil {
		t.Fatalf("write partial line: %v", err)
	}
	awaitOutstandingRead(t, trackedInput, base+uint64(len(partialLine)))

	// When
	signals <- os.Interrupt

	// Then
	awaitTextCount(t, stderr, "P> ", 3)
	if _, err := fmt.Fprintln(writer, "echo fresh"); err != nil {
		t.Fatalf("write fresh command: %v", err)
	}
	if _, err := fmt.Fprintln(writer, "exit 0"); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	closeInput()
	if err := awaitError(t, done, "idle interrupt recovery"); err != nil {
		t.Fatalf("interactive run: %v", err)
	}
	if got := stdout.String(); got != "fresh\n" {
		t.Fatalf("stdout = %q, want %q", got, "fresh\n")
	}
}
