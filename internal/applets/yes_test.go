package applets_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

var errYesWriterStopped = errors.New("yes writer stopped")

func TestDefaultRegistry_registersYes_whenLookupByName(t *testing.T) {
	// Given
	name := "yes"

	// When
	_, ok := applets.DefaultRegistry.Lookup(name)

	// Then
	if !ok {
		t.Fatal("expected yes applet to be registered")
	}
}

func TestDefaultRegistry_printsDefaultYUntilWriterFails_whenYesRunsWithoutOperands(t *testing.T) {
	// Given
	applet := lookupYes(t)
	stdout := &limitedWriter{failAfter: 3}

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, errYesWriterStopped) {
		t.Fatalf("expected bounded writer error, got %v", err)
	}
	if got := stdout.String(); got != "y\ny\ny\n" {
		t.Fatalf("expected repeated default output %q, got %q", "y\ny\ny\n", got)
	}
}

func TestDefaultRegistry_printsArgumentsJoinedBySpacesUntilWriterFails_whenYesRunsWithOperands(t *testing.T) {
	// Given
	applet := lookupYes(t)
	stdout := &limitedWriter{failAfter: 2}

	// When
	err := applet.Run(context.Background(), []string{"hello", "world"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, errYesWriterStopped) {
		t.Fatalf("expected bounded writer error, got %v", err)
	}
	if got := stdout.String(); got != "hello world\nhello world\n" {
		t.Fatalf("expected repeated operand output %q, got %q", "hello world\nhello world\n", got)
	}
}

func TestDefaultRegistry_treatsDashLikeString_whenYesRunsWithDashOperand(t *testing.T) {
	// Given
	applet := lookupYes(t)
	stdout := &limitedWriter{failAfter: 2}

	// When
	err := applet.Run(context.Background(), []string{"-n"}, &bytes.Buffer{}, stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, errYesWriterStopped) {
		t.Fatalf("expected bounded writer error, got %v", err)
	}
	if got := stdout.String(); got != "-n\n-n\n" {
		t.Fatalf("expected dash operand output %q, got %q", "-n\n-n\n", got)
	}
}

func TestDefaultRegistry_stopsWhenContextIsCancelled_whenYesRuns(t *testing.T) {
	// Given
	applet := lookupYes(t)
	ctx, cancel := context.WithCancel(context.Background())
	stdout := &cancelingWriter{cancelAfter: 2, cancel: cancel}

	// When
	err := applet.Run(ctx, nil, &bytes.Buffer{}, stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if got := stdout.String(); got != "y\ny\n" {
		t.Fatalf("expected output before cancellation %q, got %q", "y\ny\n", got)
	}
}

type limitedWriter struct {
	buffer    bytes.Buffer
	writes    int
	failAfter int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errYesWriterStopped
	}
	w.writes++
	return w.buffer.Write(p)
}

func (w *limitedWriter) String() string {
	return w.buffer.String()
}

type cancelingWriter struct {
	buffer      bytes.Buffer
	writes      int
	cancelAfter int
	cancel      context.CancelFunc
}

func (w *cancelingWriter) Write(p []byte) (int, error) {
	w.writes++
	written, err := w.buffer.Write(p)
	if w.writes == w.cancelAfter {
		w.cancel()
	}
	return written, err
}

func (w *cancelingWriter) String() string {
	return w.buffer.String()
}

var _ io.Writer = (*limitedWriter)(nil)
var _ io.Writer = (*cancelingWriter)(nil)

func lookupYes(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("yes")
	if !ok {
		t.Fatal("expected yes applet to be registered")
	}
	return applet
}
