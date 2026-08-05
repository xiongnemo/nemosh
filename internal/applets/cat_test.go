package applets_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_stopsWhenContextIsCancelled_whileCatCopies(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("expected cat applet to be registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	stdout := &catCancelingWriter{cancel: cancel}

	// When
	err := applet.Run(ctx, nil, catEndlessReader{}, stdout, io.Discard)

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

type catEndlessReader struct{}

func (catEndlessReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 'x'
	}
	return len(buffer), nil
}

type catCancelingWriter struct {
	buffer bytes.Buffer
	cancel context.CancelFunc
}

func (w *catCancelingWriter) Write(buffer []byte) (int, error) {
	written, err := w.buffer.Write(buffer)
	w.cancel()
	return written, err
}
