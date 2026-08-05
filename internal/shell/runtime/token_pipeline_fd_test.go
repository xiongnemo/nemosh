package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestTokenPipeline_serializesInheritedBorrowedFD3AcrossStages(t *testing.T) {
	// Given
	sink := newFDOverlapWriter()
	secondStarted := make(chan struct{})
	registry := applets.NewRegistry(
		fdPipelineApplet{name: "fd3-first", run: func(_ context.Context, stdout io.Writer) error {
			_, err := io.WriteString(stdout, "first")
			return err
		}},
		fdPipelineApplet{name: "fd3-second", run: func(_ context.Context, stdout io.Writer) error {
			close(secondStarted)
			_, err := io.WriteString(stdout, "second")
			return err
		}},
	)
	runtime := New(registry, Streams{})
	if err := runtime.fds.bindBorrowedWriter(3, sink); err != nil {
		t.Fatalf("bind fd3: %v", err)
	}
	done := make(chan int, 1)
	go func() { done <- runtime.RunScript(context.Background(), "fd3-first >&3 | fd3-second >&3\n") }()
	awaitFDSignal(t, sink.firstEntered, "first FD 3 write did not enter sink")

	// When
	awaitFDSignal(t, secondStarted, "second pipeline stage did not start")

	// Then
	select {
	case <-sink.overlap:
		close(sink.releaseFirst)
		awaitFDStatus(t, done)
		t.Fatal("inherited borrowed FD 3 writes overlapped")
	case <-time.After(100 * time.Millisecond):
		close(sink.releaseFirst)
	}
	awaitFDStatus(t, done)
}

func TestTokenPipeline_inheritsFD3WithStageLocalCloseIsolation(t *testing.T) {
	// Given
	var fd3 bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{})
	if err := runtime.fds.bindBorrowedWriter(3, &fd3); err != nil {
		t.Fatalf("bind fd3: %v", err)
	}

	// When
	status := runtime.RunScript(context.Background(), "echo hidden 3>&- 1>&3 | echo sibling >&3\necho parent >&3\n")

	// Then
	if status != 0 || fd3.String() != "sibling\nparent\n" {
		t.Fatalf("status=%d fd3=%q", status, fd3.String())
	}
}

func TestTokenPipeline_inheritsFD3WithStageLocalRebindIsolation(t *testing.T) {
	// Given
	var fd3 bytes.Buffer
	stageOut := filepath.ToSlash(filepath.Join(t.TempDir(), "stage.txt"))
	runtime := New(applets.DefaultRegistry, Streams{})
	if err := runtime.fds.bindBorrowedWriter(3, &fd3); err != nil {
		t.Fatalf("bind fd3: %v", err)
	}

	// When
	status := runtime.RunScript(context.Background(), "echo stage 3>"+stageOut+" >&3 | echo sibling >&3\necho parent >&3\n")

	// Then
	contents, err := os.ReadFile(stageOut)
	if err != nil {
		t.Fatalf("read stage output: %v", err)
	}
	if status != 0 || string(contents) != "stage\n" || fd3.String() != "sibling\nparent\n" {
		t.Fatalf("status=%d stage=%q fd3=%q", status, contents, fd3.String())
	}
}

type fdPipelineApplet struct {
	name string
	run  func(context.Context, io.Writer) error
}

func (a fdPipelineApplet) Name() string { return a.name }

func (a fdPipelineApplet) Run(ctx context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
	return a.run(ctx, stdout)
}

type fdOverlapWriter struct {
	mu           sync.Mutex
	active       bool
	firstEntered chan struct{}
	releaseFirst chan struct{}
	overlap      chan struct{}
	firstOnce    sync.Once
	overlapOnce  sync.Once
}

func newFDOverlapWriter() *fdOverlapWriter {
	return &fdOverlapWriter{
		firstEntered: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		overlap:      make(chan struct{}),
	}
}

func (w *fdOverlapWriter) Write(buffer []byte) (int, error) {
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

func awaitFDSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}

func awaitFDStatus(t *testing.T, done <-chan int) {
	t.Helper()
	select {
	case status := <-done:
		if status != 0 {
			t.Fatalf("status=%d", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not return")
	}
}
