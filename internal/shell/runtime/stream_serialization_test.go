package runtime_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_serializesBorrowedStdoutAndStderrAcrossPipelineStages(t *testing.T) {
	// Given
	sink := newOverlapWriter()
	secondMayWrite := make(chan struct{})
	secondAttempted := make(chan struct{})
	registry := applets.NewRegistry(
		pipelineApplet{name: "write-stderr", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
			return writeText(stderr, "first")
		}},
		pipelineApplet{name: "write-stdout", run: func(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			<-secondMayWrite
			close(secondAttempted)
			return writeText(stdout, "second")
		}},
	)
	rt := runtime.New(registry, runtime.Streams{Stdout: sink, Stderr: sink})
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(context.Background(), "write-stderr | write-stdout\n") }()
	awaitSignal(t, sink.firstEntered, "first write did not enter sink")

	// When
	close(secondMayWrite)
	awaitSignal(t, secondAttempted, "second write was not attempted")

	// Then
	select {
	case <-sink.overlap:
		close(sink.releaseFirst)
		awaitStatusValue(t, done, 0)
		t.Fatal("borrowed stdout and stderr writes overlapped")
	case <-time.After(100 * time.Millisecond):
		close(sink.releaseFirst)
	}
	awaitStatusValue(t, done, 0)
}

type overlapWriter struct {
	mu           sync.Mutex
	active       bool
	firstEntered chan struct{}
	releaseFirst chan struct{}
	overlap      chan struct{}
	firstOnce    sync.Once
	overlapOnce  sync.Once
}

func newOverlapWriter() *overlapWriter {
	return &overlapWriter{
		firstEntered: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		overlap:      make(chan struct{}),
	}
}

func (w *overlapWriter) Write(buffer []byte) (int, error) {
	w.mu.Lock()
	overlapped := w.active
	if !overlapped {
		w.active = true
	}
	w.mu.Unlock()
	if overlapped {
		w.overlapOnce.Do(func() { close(w.overlap) })
		return len(buffer), nil
	}
	w.firstOnce.Do(func() { close(w.firstEntered) })
	<-w.releaseFirst
	w.mu.Lock()
	w.active = false
	w.mu.Unlock()
	return len(buffer), nil
}

func writeText(writer io.Writer, text string) error {
	_, err := io.WriteString(writer, text)
	return err
}

func awaitStatusValue(t *testing.T, done <-chan int, want int) {
	t.Helper()
	select {
	case status := <-done:
		if status != want {
			t.Fatalf("status=%d want=%d", status, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not return")
	}
}
