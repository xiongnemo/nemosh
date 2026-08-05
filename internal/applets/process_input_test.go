package applets_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestP05WaveA_streamingInputApplets_useSharedProcessInputAndCloseOnce(t *testing.T) {
	tests := []struct {
		name string
		args []string
		data string
		want string
	}{
		{name: "cat", args: []string{"cat", "/dev/test"}, data: "alpha\n", want: "alpha\n"},
		{name: "wc", args: []string{"wc", "-l", "/dev/test"}, data: "alpha\n", want: "1 /dev/test\n"},
		{name: "head", args: []string{"head", "-n", "1", "/dev/test"}, data: "alpha\nbeta\n", want: "alpha\n"},
		{name: "tail", args: []string{"tail", "-n", "1", "/dev/test"}, data: "alpha\nbeta\n", want: "beta\n"},
		{name: "grep", args: []string{"grep", "alpha", "/dev/test"}, data: "alpha\n", want: "alpha\n"},
		{name: "cut", args: []string{"cut", "-d", " ", "-f", "1", "/dev/test"}, data: "alpha beta\n", want: "alpha\n"},
		{name: "sort", args: []string{"sort", "/dev/test"}, data: "beta\nalpha\n", want: "alpha\nbeta\n"},
		{name: "uniq", args: []string{"uniq", "/dev/test"}, data: "alpha\nalpha\n", want: "alpha\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			input := &countingReadCloser{Reader: bytes.NewBufferString(test.data)}
			view := processInputTestView{input: input}
			ctx := applets.WithProcessView(context.Background(), view)
			var stdout bytes.Buffer

			// When
			err := runRegisteredApplet(ctx, test.args, bytes.NewReader(nil), &stdout)

			// Then
			if err != nil {
				t.Fatalf("run %s: %v", test.args[0], err)
			}
			if got := stdout.String(); got != test.want {
				t.Fatalf("%s output: got %q want %q", test.args[0], got, test.want)
			}
			if input.closes != 1 {
				t.Fatalf("%s close count: got %d want 1", test.args[0], input.closes)
			}
		})
	}
}

func TestP05WaveA_streamingApplets_preserveReadAndCloseErrors(t *testing.T) {
	readErr := errors.New("read failed")
	closeErr := errors.New("close failed")
	tests := [][]string{
		{"cat", "/dev/test"},
		{"wc", "/dev/test"},
		{"head", "/dev/test"},
		{"tail", "/dev/test"},
		{"grep", "match", "/dev/test"},
	}

	for _, args := range tests {
		t.Run(args[0], func(t *testing.T) {
			// Given
			input := &failingReadCloser{readErr: readErr, closeErr: closeErr}
			view := processInputTestView{input: input}
			ctx := applets.WithProcessView(context.Background(), view)

			// When
			err := runRegisteredApplet(ctx, args, bytes.NewReader(nil), io.Discard)

			// Then
			if !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
				t.Fatalf("%s error=%v; want joined read and close errors", args[0], err)
			}
		})
	}
}

func TestFinalSecurity_streamingInputApplets_stopReadingWhenContextCanceled(t *testing.T) {
	readAfterCancelErr := errors.New("underlying reader called after cancellation")
	tests := [][]string{
		{"cat", "/dev/test"},
		{"wc", "/dev/test"},
		{"head", "-n", "1000000", "/dev/test"},
		{"tail", "-n", "10", "/dev/test"},
		{"grep", "alpha", "/dev/test"},
		{"cut", "-d", " ", "-f", "1", "/dev/test"},
		{"sort", "/dev/test"},
		{"uniq", "/dev/test"},
	}

	for _, args := range tests {
		t.Run(args[0], func(t *testing.T) {
			// Given
			ctx, cancel := context.WithCancel(context.Background())
			reader := &cancelAfterReader{
				cancel:             cancel,
				readsBeforeCancel:  3,
				data:               []byte("alpha beta\n"),
				readAfterCancelErr: readAfterCancelErr,
			}
			view := processInputTestView{input: reader}
			ctx = applets.WithProcessView(ctx, view)

			// When
			err := runRegisteredApplet(ctx, args, bytes.NewReader(nil), io.Discard)

			// Then
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s cancellation error: got %v want %v", args[0], err, context.Canceled)
			}
			if errors.Is(err, readAfterCancelErr) {
				t.Fatalf("%s read underlying input after cancellation: %v", args[0], err)
			}
			if got := reader.reads.Load(); got != 3 {
				t.Fatalf("%s underlying read count: got %d want 3", args[0], got)
			}
			if got := reader.closes.Load(); got != 1 {
				t.Fatalf("%s input close count: got %d want 1", args[0], got)
			}
		})
	}
}

func TestFinalSecurity_cutSortUniq_preserveDeadlineExceededBetweenReads(t *testing.T) {
	closeErr := errors.New("close failed")
	readAfterDeadlineErr := errors.New("underlying reader called after deadline")
	tests := [][]string{
		{"cut", "-d", " ", "-f", "1", "/dev/test"},
		{"sort", "/dev/test"},
		{"uniq", "/dev/test"},
	}

	for _, args := range tests {
		t.Run(args[0], func(t *testing.T) {
			// Given
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			reader := &deadlineBetweenReadsCloser{
				ctx:                  ctx,
				data:                 []byte("alpha beta\n"),
				readAfterDeadlineErr: readAfterDeadlineErr,
				closeErr:             closeErr,
			}
			view := processInputTestView{input: reader}
			ctx = applets.WithProcessView(ctx, view)

			// When
			err := runRegisteredApplet(ctx, args, bytes.NewReader(nil), io.Discard)

			// Then
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%s deadline error: got %v want %v", args[0], err, context.DeadlineExceeded)
			}
			if !errors.Is(err, closeErr) {
				t.Fatalf("%s close error: got %v want joined %v", args[0], err, closeErr)
			}
			if errors.Is(err, readAfterDeadlineErr) {
				t.Fatalf("%s read underlying input after deadline: %v", args[0], err)
			}
			if got := reader.reads.Load(); got != 1 {
				t.Fatalf("%s underlying read count: got %d want 1", args[0], got)
			}
			if got := reader.closes.Load(); got != 1 {
				t.Fatalf("%s input close count: got %d want 1", args[0], got)
			}
		})
	}
}

func TestFinalSecurity_streamingInputApplets_stopReadingCanceledStdin(t *testing.T) {
	readAfterCancelErr := errors.New("stdin reader called after cancellation")
	tests := [][]string{
		{"cat"},
		{"wc"},
		{"head", "-n", "1000000"},
		{"tail", "-n", "10"},
		{"grep", "alpha"},
		{"cut", "-d", " ", "-f", "1"},
		{"sort"},
		{"uniq"},
	}
	for _, args := range tests {
		t.Run(args[0], func(t *testing.T) {
			// Given
			ctx, cancel := context.WithCancel(context.Background())
			reader := &cancelAfterReader{
				cancel:             cancel,
				readsBeforeCancel:  3,
				data:               []byte("alpha beta\n"),
				readAfterCancelErr: readAfterCancelErr,
			}

			// When
			err := runRegisteredApplet(ctx, args, reader, io.Discard)

			// Then
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s stdin cancellation error: got %v want %v", args[0], err, context.Canceled)
			}
			if errors.Is(err, readAfterCancelErr) {
				t.Fatalf("%s read stdin after cancellation: %v", args[0], err)
			}
			if got := reader.reads.Load(); got != 3 {
				t.Fatalf("%s stdin read count: got %d want 3", args[0], got)
			}
		})
	}
}

type processInputTestView struct {
	input io.ReadCloser
}

func (processInputTestView) WorkingDirectory() string        { return "." }
func (processInputTestView) Environ() []string               { return nil }
func (processInputTestView) LookupEnv(string) (string, bool) { return "", false }
func (processInputTestView) ResolvePath(path string) string  { return path }
func (v processInputTestView) OpenProcessInput(path string) (io.ReadCloser, error) {
	if path != "/dev/test" {
		return nil, fmt.Errorf("unexpected process input %q", path)
	}
	return v.input, nil
}

type countingReadCloser struct {
	io.Reader
	closes int
}

type failingReadCloser struct {
	readErr  error
	closeErr error
}

type cancelAfterReader struct {
	cancel             context.CancelFunc
	readsBeforeCancel  int32
	data               []byte
	readAfterCancelErr error
	reads              atomic.Int32
	closes             atomic.Int32
}

type deadlineBetweenReadsCloser struct {
	ctx                  context.Context
	data                 []byte
	readAfterDeadlineErr error
	closeErr             error
	reads                atomic.Int32
	closes               atomic.Int32
}

func (r *failingReadCloser) Read([]byte) (int, error) { return 0, r.readErr }
func (r *failingReadCloser) Close() error             { return r.closeErr }

func (r *countingReadCloser) Close() error {
	r.closes++
	return nil
}

func (r *cancelAfterReader) Read(buffer []byte) (int, error) {
	read := r.reads.Add(1)
	if read > r.readsBeforeCancel {
		return 0, r.readAfterCancelErr
	}
	count := copy(buffer, r.data)
	if read == r.readsBeforeCancel {
		r.cancel()
	}
	return count, nil
}

func (r *cancelAfterReader) Close() error {
	r.closes.Add(1)
	return nil
}

func (r *deadlineBetweenReadsCloser) Read(buffer []byte) (int, error) {
	if r.reads.Add(1) > 1 {
		return 0, r.readAfterDeadlineErr
	}
	<-r.ctx.Done()
	return copy(buffer, r.data), nil
}

func (r *deadlineBetweenReadsCloser) Close() error {
	r.closes.Add(1)
	return r.closeErr
}
