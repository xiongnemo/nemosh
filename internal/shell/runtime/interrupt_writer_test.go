package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestInterruptibleWriter_stillNamesTheFileItEndsAt is the half that is easy to break
// silently.
//
// Everything an applet writes to already goes through a descriptorWriter, so the naive
// `stdout.(*os.File)` is false inside the shell and the tree answers the question with
// TerminalFile() instead. fd_stream.go records what happens when that is got wrong: `ls`
// laid out no columns on a terminal and `--color=auto` coloured nothing, and both looked
// like a missing feature rather than a question asked of the wrong object. A wrapper put
// in front of stdout has to pass the question through, and nothing else would notice if
// it did not.
func TestInterruptibleWriter_stillNamesTheFileItEndsAt(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wrapped := interruptibleWriter{writer: file, done: ctx.Done(), cause: ctx.Err}

	if got := terminalFileOf(wrapped); got != file {
		t.Errorf("terminalFileOf answered %v, not the file the chain ends at", got)
	}
}

func TestInterruptibleWriter_writesUntilCancelledAndThenFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var sink countingSink
	wrapped := interruptibleWriter{writer: &sink, done: ctx.Done(), cause: ctx.Err}

	if _, err := wrapped.Write([]byte("before")); err != nil {
		t.Fatalf("a live context refused a write: %v", err)
	}
	cancel()
	n, err := wrapped.Write([]byte("after"))
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("after cancellation the write answered (%d, %v), want (0, context.Canceled)", n, err)
	}
	// The error has to be the context's own, because runCommand answers 130 silently for
	// exactly that error and reports anything else as a failed command.
	if sink.written != len("before") {
		t.Errorf("the sink received %d bytes, want only what was written before cancelling", sink.written)
	}
}

type countingSink struct{ written int }

func (s *countingSink) Write(p []byte) (int, error) {
	s.written += len(p)
	return len(p), nil
}
