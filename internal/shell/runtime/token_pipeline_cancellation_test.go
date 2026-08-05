package runtime_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_tokenPipelineCancellationUnblocksNonCooperativePipeRead(t *testing.T) {
	// Given
	readerStarted := make(chan struct{})
	readerMayRead := make(chan struct{})
	producerStopped := make(chan struct{})
	readerStopped := make(chan struct{})
	registry := applets.NewRegistry(
		pipelineApplet{name: "hold-writer", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			<-readerStarted
			close(readerMayRead)
			<-ctx.Done()
			<-readerStopped
			close(producerStopped)
			return ctx.Err()
		}},
		pipelineApplet{name: "blocked-reader", run: func(_ context.Context, _ []string, stdin io.Reader, _ io.Writer, _ io.Writer) error {
			close(readerStarted)
			<-readerMayRead
			_, err := stdin.Read(make([]byte, 1))
			close(readerStopped)
			return err
		}},
	)
	rt := runtime.New(registry, runtime.Streams{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "hold-writer | blocked-reader\n") }()
	awaitSignal(t, readerStarted, "reader did not start")

	// When
	cancel()

	// Then
	awaitSignal(t, readerStopped, "pipe read remained blocked")
	awaitSignal(t, producerStopped, "producer did not stop after pipe read returned")
	awaitStatus(t, done)
}

func TestRuntime_tokenPipelineCancellationUnblocksNonCooperativeFullPipeWrite(t *testing.T) {
	// Given
	readerHeld := make(chan struct{})
	readerStopped := make(chan struct{})
	writerStopped := make(chan struct{})
	registry := applets.NewRegistry(
		pipelineApplet{name: "fill-pipe", run: func(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			<-readerHeld
			_, err := io.Copy(stdout, endlessReader{})
			close(writerStopped)
			return err
		}},
		pipelineApplet{name: "hold-reader", run: func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			close(readerHeld)
			<-ctx.Done()
			<-writerStopped
			close(readerStopped)
			return ctx.Err()
		}},
	)
	rt := runtime.New(registry, runtime.Streams{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "fill-pipe | hold-reader\n") }()
	awaitSignal(t, readerHeld, "reader did not retain pipe")

	// When
	cancel()

	// Then
	awaitSignal(t, readerStopped, "reader did not stop")
	awaitSignal(t, writerStopped, "full pipe write remained blocked")
	awaitStatus(t, done)
}

type endlessReader struct{}

func (endlessReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 'x'
	}
	return len(buffer), nil
}

func awaitSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}

func awaitStatus(t *testing.T, done <-chan int) {
	t.Helper()
	select {
	case status := <-done:
		if status == 0 {
			t.Fatal("expected cancellation failure status")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not return after cancellation")
	}
}
