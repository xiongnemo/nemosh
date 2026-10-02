package applets_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
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
	assertWholeLines(t, stdout.String(), "y\n", 3)
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
	assertWholeLines(t, stdout.String(), "hello world\n", 2)
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
	assertWholeLines(t, stdout.String(), "-n\n", 2)
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
	assertWholeLines(t, stdout.String(), "y\n", 2)
}

// assertWholeLines checks that got is unit repeated, at least least times. yes writes a block of
// whole lines at a time, so a writer that stops after some writes holds that many blocks.
func assertWholeLines(t *testing.T, got, unit string, least int) {
	t.Helper()
	count := strings.Count(got, unit)
	if count < least || got != strings.Repeat(unit, count) {
		t.Fatalf("expected at least %d whole lines of %q, got %d bytes starting %q", least, unit, len(got), got[:min(len(got), 40)])
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
