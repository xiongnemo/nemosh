package applets

import (
	"context"
	"io"
	"os"
	"testing"
)

// **`less` inside the shell has to ask the stream chain, not the first link.**
//
// Reported from a real terminal: `less file` printed the whole file and returned, like
// `cat`. It never tried to page.
//
// Everything an applet writes to goes through a descriptorWriter rather than os.Stdout, so
// `stdout.(*os.File)` is false inside the shell -- fd_stream.go says exactly that, and
// ls_columns.go already answers the question properly with stdoutIsTerminal. lessCanPage
// was a second, wrong copy of it, and its `return false` had a comment accepting the wrong
// answer rather than questioning it. So `less` concluded there was no terminal every time
// it was run from the prompt, and fell back to printing plainly.
//
// Same shape on the input side: lessInputIsTerminal decides whether `less` with no operands
// refuses instead of reading the keyboard as if it were a file, and it was answering false
// for the same reason -- which turns that refusal into a hang, the shape bc and dc had.
// `top` gets this right through LeaseStdinFile, and `less` now asks the same way.
//
// This is the fifth time a wrapper chain has been answered at the wrong hop: TerminalFile
// on descriptorWriter, then synchronizedWriter having to forward it, then LeaseStdinFile
// for `top`, then readWithContext for the cancellation, now this.
//
// The tests assert that the chain is *consulted*, not what the answer is: a runner has no
// terminal, so asking whether the answer is true would only prove the runner has no
// terminal.

// spyTerminalWriter answers the TerminalFile question and records having been asked.
type spyTerminalWriter struct {
	asked bool
	file  *os.File
}

func (w *spyTerminalWriter) Write(p []byte) (int, error) { return len(p), nil }

func (w *spyTerminalWriter) TerminalFile() *os.File {
	w.asked = true
	return w.file
}

func TestLessCanPage_asksTheStreamWhichFileItEndsAt(t *testing.T) {
	spy := &spyTerminalWriter{}
	lessCanPage(spy)
	if !spy.asked {
		t.Error("lessCanPage did not ask the stream which file it ends at, so inside the shell " +
			"-- where stdout is never an *os.File -- it can only ever answer no, and `less` " +
			"prints the file instead of paging it")
	}
}

// spyLeasingReader answers the stdin-file question and records having been asked.
type spyLeasingReader struct {
	asked bool
	file  *os.File
}

func (r *spyLeasingReader) Read([]byte) (int, error) { return 0, io.EOF }

func (r *spyLeasingReader) LeaseStdinFile(context.Context) (*os.File, func(), bool) {
	r.asked = true
	return r.file, func() {}, r.file != nil
}

func TestLessInputIsTerminal_asksTheStreamForItsFile(t *testing.T) {
	spy := &spyLeasingReader{}
	lessInputIsTerminal(context.Background(), spy)
	if !spy.asked {
		t.Error("lessInputIsTerminal did not ask the stream for its file, so `less` with no " +
			"operands reads the keyboard as though it were a file instead of refusing")
	}
}

// TestLessCanPage_saysNoForSomethingThatIsNotATerminal keeps the fix from going too far:
// a pipe or a file is still not a pager, which is what makes `less file | cat` behave.
func TestLessCanPage_saysNoForSomethingThatIsNotATerminal(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "not-a-terminal-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	if lessCanPage(file) {
		t.Error("a regular file was taken for a terminal, so `less > out` would try to page")
	}
	if lessCanPage(&spyTerminalWriter{file: file}) {
		t.Error("a stream ending at a regular file was taken for a terminal")
	}
	if lessCanPage(io.Discard) {
		t.Error("a stream that names no file at all was taken for a terminal")
	}
}

// TestIsTerminal_asksTheStreamWhichFileItEndsAt is `[ -t 1 ]`, which has the same bug and
// is the more consequential one: it is how a script asks whether it is talking to a person
// before deciding to use colour, and inside the shell it always answered no.
func TestIsTerminal_asksTheStreamWhichFileItEndsAt(t *testing.T) {
	for _, testcase := range []struct {
		name       string
		descriptor string
		streams    func(*spyTerminalWriter, *spyLeasingReader) [3]any
		asked      func(*spyTerminalWriter, *spyLeasingReader) bool
	}{
		{
			name:       "stdout",
			descriptor: "1",
			streams: func(w *spyTerminalWriter, r *spyLeasingReader) [3]any {
				return [3]any{r, w, io.Discard}
			},
			asked: func(w *spyTerminalWriter, _ *spyLeasingReader) bool { return w.asked },
		},
		{
			name:       "stderr",
			descriptor: "2",
			streams: func(w *spyTerminalWriter, r *spyLeasingReader) [3]any {
				return [3]any{r, io.Discard, w}
			},
			asked: func(w *spyTerminalWriter, _ *spyLeasingReader) bool { return w.asked },
		},
		{
			name:       "stdin",
			descriptor: "0",
			streams: func(w *spyTerminalWriter, r *spyLeasingReader) [3]any {
				return [3]any{r, w, io.Discard}
			},
			asked: func(_ *spyTerminalWriter, r *spyLeasingReader) bool { return r.asked },
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			writer, reader := &spyTerminalWriter{}, &spyLeasingReader{}
			evaluator := &testEvaluator{streams: testcase.streams(writer, reader)}
			if _, err := evaluator.isTerminal(testcase.descriptor); err != nil {
				t.Fatalf("isTerminal: %v", err)
			}
			if !testcase.asked(writer, reader) {
				t.Errorf("`test -t %s` did not ask the stream which file it ends at, so inside "+
					"the shell -- where a stream is never an *os.File -- it answers no on a real "+
					"terminal", testcase.descriptor)
			}
		})
	}
}
