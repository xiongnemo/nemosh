package applets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Who owns cancellation, settled.
//
// The v1 ledger carried this as a debt on the grounds that copyWithContext is
// redundant: contextApplet in registry.go already wraps every applet's stdin, so
// cat appeared to have two prechecking wrappers, and the suggested remedy was to
// collapse it to plain io.Copy.
//
// That premise is half right, and the half that is wrong is the half that
// matters. It holds for **stdin**, where the second wrap is a harmless extra
// hop. It does not hold for an **operand**: `cat file.txt` hands
// OpenProcessInput's *os.File straight to the copy, and a file knows nothing
// about a context. Collapsing to io.Copy there would mean Ctrl-C could not
// interrupt `cat` on a large file -- silently, and only for the case a user is
// most likely to hit.
//
// So copyWithContext stays, and these tests say what each layer is for: the
// registry makes stdin cancellable, and copyWithContext makes anything else
// cancellable. Removing either one fails one of them.

// blockingReader never ends and knows nothing about a context, which is what a
// file looks like from here.
type blockingReader struct{}

func (blockingReader) Read(buffer []byte) (int, error) {
	time.Sleep(time.Millisecond)
	buffer[0] = 'x'
	return 1, nil
}

// An applet constructed directly -- without the registry's wrapper -- must still
// honour the context, because that is the only thing standing between a cancelled
// Ctrl-C and an unbounded read of an operand.
func TestCancellation_appletHonoursContextWithoutTheRegistryWrapper(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	// When: catApplet directly, so nothing has wrapped the reader
	done := make(chan error, 1)
	go func() {
		done <- catApplet{}.Run(ctx, nil, blockingReader{}, io.Discard, io.Discard)
	}()

	// Then
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cat did not stop when the context was cancelled")
	}
}

// And the same for an operand, which is the case the registry's wrapper cannot
// reach: it wraps stdin only.
func TestCancellation_reachesAFileOperand(t *testing.T) {
	// Given: a file large enough that the copy cannot finish instantly
	directory := t.TempDir()
	path := filepath.Join(directory, "big.txt")
	block := bytes.Repeat([]byte("0123456789abcdef"), 1024)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 512 {
		if _, err := file.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	// When: cancelled before it starts, which is the deterministic form of the
	// same question -- a context already done must stop the copy at once.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	applet, ok := DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("cat is not registered")
	}
	var stdout bytes.Buffer
	err = applet.Run(WithProcessView(ctx, cancellationTestView{cwd: directory}), []string{"big.txt"},
		bytes.NewReader(nil), &stdout, io.Discard)

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if stdout.Len() > len(block) {
		t.Fatalf("copied %d bytes after cancellation", stdout.Len())
	}
}

// The registry's half: stdin arrives cancellable even for an applet that reads it
// with no context of its own.
func TestCancellation_registryWrapsStdin(t *testing.T) {
	// Given
	applet, ok := DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("cat is not registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	err := applet.Run(ctx, nil, blockingReader{}, io.Discard, io.Discard)

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

type cancellationTestView struct{ cwd string }

func (v cancellationTestView) WorkingDirectory() string             { return v.cwd }
func (v cancellationTestView) LookupEnv(string) (string, bool)      { return "", false }
func (v cancellationTestView) Environ() []string                    { return nil }
func (v cancellationTestView) ResolvePath(path string) string       { return filepath.Join(v.cwd, path) }
func (v cancellationTestView) LookupVariable(string) (string, bool) { return "", false }
