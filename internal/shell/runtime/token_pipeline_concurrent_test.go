package runtime_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

type pipelineApplet struct {
	name string
	run  func(context.Context, []string, io.Reader, io.Writer, io.Writer) error
}

func (a pipelineApplet) Name() string { return a.name }

func (a pipelineApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return a.run(ctx, args, stdin, stdout, stderr)
}

func TestRuntime_tokenPipelineStartsAllStagesBeforeProducerContinues(t *testing.T) {
	// Given
	consumerStarted := make(chan struct{})
	registry := applets.NewRegistry(
		pipelineApplet{name: "gated-producer", run: func(ctx context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			select {
			case <-consumerStarted:
				_, err := io.WriteString(stdout, "streamed\n")
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}},
		pipelineApplet{name: "start-consumer", run: func(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
			close(consumerStarted)
			_, err := io.Copy(stdout, stdin)
			return err
		}},
	)
	var stdout bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// When
	status := rt.RunScript(ctx, "gated-producer | start-consumer\n")

	// Then
	if status != 0 || stdout.String() != "streamed\n" {
		t.Fatalf("status=%d stdout=%q context=%v", status, stdout.String(), ctx.Err())
	}
}

func TestRuntime_tokenPipelineStreamsOutputBeyondPipeCapacity(t *testing.T) {
	// Given
	const size = 2 << 20
	registry := applets.NewRegistry(
		pipelineApplet{name: "large-producer", run: func(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			_, err := io.CopyN(stdout, strings.NewReader(strings.Repeat("x", size)), size)
			return err
		}},
		pipelineApplet{name: "byte-counter", run: func(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
			count, err := io.Copy(io.Discard, stdin)
			if err == nil {
				_, err = fmt.Fprintln(stdout, count)
			}
			return err
		}},
	)
	var stdout bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// When
	status := rt.RunScript(ctx, "large-producer | byte-counter\n")

	// Then
	if status != 0 || stdout.String() != fmt.Sprintf("%d\n", size) {
		t.Fatalf("status=%d stdout=%q context=%v", status, stdout.String(), ctx.Err())
	}
}

func TestRuntime_tokenPipelineClosesWriterToDeliverEOF(t *testing.T) {
	// Given
	registry := applets.NewRegistry(
		pipelineApplet{name: "one-record", run: func(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
			_, err := io.WriteString(stdout, "record")
			return err
		}},
		pipelineApplet{name: "await-eof", run: func(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
			contents, err := io.ReadAll(stdin)
			if err == nil {
				_, err = stdout.Write(contents)
			}
			return err
		}},
	)
	var stdout bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// When
	status := rt.RunScript(ctx, "one-record | await-eof\n")

	// Then
	if status != 0 || stdout.String() != "record" {
		t.Fatalf("status=%d stdout=%q context=%v", status, stdout.String(), ctx.Err())
	}
}

func TestRuntime_tokenPipelineAllowsEarlyReaderExitWithoutHanging(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// When
	status := rt.RunScript(ctx, "yes | head -n 1\n")

	// Then
	if status != 0 || stdout.String() != "y\n" || stderr.String() != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q context=%v", status, stdout.String(), stderr.String(), ctx.Err())
	}
}

func TestRuntime_tokenPipelineReportsUnrelatedUpstreamErrors(t *testing.T) {
	// Given
	registry := applets.NewRegistry(
		pipelineApplet{name: "unrelated-error", run: func(_ context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			return errors.New("unrelated failure")
		}},
		pipelineApplet{name: "drain", run: func(_ context.Context, _ []string, stdin io.Reader, _ io.Writer, _ io.Writer) error {
			_, err := io.Copy(io.Discard, stdin)
			return err
		}},
	)
	var stderr bytes.Buffer
	rt := runtime.New(registry, runtime.Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "unrelated-error | drain\n")

	// Then
	if status != 0 || stderr.String() != "unrelated-error: unrelated failure\n" {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
}

func TestRuntime_tokenPipelinePreservesStderrAndExplicitRedirectPrecedence(t *testing.T) {
	// Given
	registry := applets.NewRegistry(
		pipelineApplet{name: "both-streams", run: func(_ context.Context, _ []string, _ io.Reader, stdout, stderr io.Writer) error {
			_, outErr := io.WriteString(stdout, "pipe\n")
			_, errErr := io.WriteString(stderr, "diagnostic\n")
			return errors.Join(outErr, errErr)
		}},
		pipelineApplet{name: "copy", run: func(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
			_, err := io.Copy(stdout, stdin)
			return err
		}},
	)
	var stdout, stderr bytes.Buffer
	out := filepath.ToSlash(filepath.Join(t.TempDir(), "redirected.txt"))
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "both-streams >"+out+" | copy\n")

	// Then
	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read redirect: %v", err)
	}
	if status != 0 || string(contents) != "pipe\n" || stdout.String() != "" || stderr.String() != "diagnostic\n" {
		t.Fatalf("status=%d file=%q stdout=%q stderr=%q", status, contents, stdout.String(), stderr.String())
	}
}

func TestRuntime_tokenPipelineSnapshotsStateBeforeLaunch(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "VALUE=parent\nVALUE=stage true | true\necho $VALUE\n")

	// Then
	if status != 0 || stdout.String() != "parent\n" {
		t.Fatalf("status=%d stdout=%q", status, stdout.String())
	}
}

func TestRuntime_tokenPipelineGivesEveryStageIncomingSavedStatus(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "false\ntrue | exit\n")

	// Then
	if status != 1 {
		t.Fatalf("status=%d", status)
	}
}

func TestRuntime_tokenPipelineDoesNotPropagateStageControlToParent(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "true | exit 7\necho parent\n")

	// Then
	if status != 0 || stdout.String() != "parent\n" {
		t.Fatalf("status=%d stdout=%q", status, stdout.String())
	}
}

func TestRuntime_tokenPipelineSelectsLexicalStatusesWithPipefail(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	defaultStatus := rt.RunScript(context.Background(), "false | true | false | true\n")
	pipefailStatus := rt.RunScript(context.Background(), "set -o pipefail\nfalse | true | false | true\n")

	// Then
	if defaultStatus != 0 || pipefailStatus != 1 {
		t.Fatalf("default=%d pipefail=%d", defaultStatus, pipefailStatus)
	}
}

func TestRuntime_tokenPipelineCancellationStopsAndCleansAllStages(t *testing.T) {
	// Given
	var once sync.Once
	stopped := make(chan struct{})
	block := func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		<-ctx.Done()
		once.Do(func() { close(stopped) })
		return ctx.Err()
	}
	registry := applets.NewRegistry(pipelineApplet{name: "block", run: block})
	rt := runtime.New(registry, runtime.Streams{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() { done <- rt.RunScript(ctx, "block | block\n") }()

	// When
	cancel()

	// Then
	select {
	case status := <-done:
		if status == 0 {
			t.Fatal("expected cancellation failure status")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not return after cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline stages did not observe cancellation")
	}
}
