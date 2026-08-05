package applets

import (
	"context"
	"errors"
	"io"
	"testing"
)

// contextualInput reports the context it was actually handed, so a wrapper that
// silently substitutes a different one is observable.
type contextualInput struct {
	closed bool
}

func (*contextualInput) Read(buffer []byte) (int, error) {
	return 0, errors.New("plain Read must not be used when ReadContext is available")
}

func (*contextualInput) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	buffer[0] = 'x'
	return 1, nil
}

func (i *contextualInput) Close() error {
	i.closed = true
	return nil
}

type contextualInputView struct {
	legacyPathTestView
	input io.ReadCloser
}

func (v contextualInputView) OpenProcessInput(string) (io.ReadCloser, error) {
	return v.input, nil
}

func TestOpenProcessInput_readContextHonorsTheCallerContextNotTheOpenContext(t *testing.T) {
	// Given an input opened with a live context, then read through a canceled one.
	input := &contextualInput{}
	reader, err := OpenProcessInput(context.Background(), contextualInputView{input: input}, "/dev/stdin")
	if err != nil {
		t.Fatalf("open process input: %v", err)
	}
	contextual, ok := reader.(interface {
		ReadContext(context.Context, []byte) (int, error)
	})
	if !ok {
		t.Fatal("process input dropped the ReadContext capability")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	buffer := make([]byte, 1)
	count, readErr := contextual.ReadContext(canceled, buffer)

	// Then the caller's canceled context must win over the one captured at open time.
	if count != 0 || !errors.Is(readErr, context.Canceled) {
		t.Fatalf("contextual read: count=%d error=%v", count, readErr)
	}
}

func TestOpenProcessInput_plainReadUsesTheContextCapturedAtOpen(t *testing.T) {
	// Given an input opened with a context that is canceled afterwards.
	openCtx, cancel := context.WithCancel(context.Background())
	input := &contextualInput{}
	reader, err := OpenProcessInput(openCtx, contextualInputView{input: input}, "/dev/stdin")
	if err != nil {
		t.Fatalf("open process input: %v", err)
	}
	buffer := make([]byte, 1)
	if count, readErr := reader.Read(buffer); count != 1 || readErr != nil {
		t.Fatalf("read before cancellation: count=%d error=%v", count, readErr)
	}

	// When
	cancel()
	count, readErr := reader.Read(buffer)

	// Then
	if count != 0 || !errors.Is(readErr, context.Canceled) {
		t.Fatalf("read after cancellation: count=%d error=%v", count, readErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !input.closed {
		t.Fatal("close did not reach the underlying input")
	}
}
