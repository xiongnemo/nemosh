package applets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// The one property a calculator at a keyboard has to have: **an answer before the input
// ends**.
//
// Every other test here feeds a whole program and reads the output afterwards, which a
// strings.Reader makes indistinguishable from an interactive session -- so all of them
// passed while `bc` read standard input to the end before evaluating anything, and an
// interactive `bc` printed nothing and appeared to hang. A terminal has no end.
//
// These tests hold the input open on purpose. With the old code they time out rather than
// fail on a wrong answer, which is exactly the shape of the bug.

// lockedBuffer is written by the applet's goroutine and read by the test's, so it needs a
// lock of its own -- the race detector runs over this suite.
type lockedBuffer struct {
	mutex sync.Mutex
	text  bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.text.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.text.String()
}

// waitForOutput waits until the applet has written what was expected, or gives up.
func waitForOutput(t *testing.T, out *lockedBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("nothing containing %q arrived while the input was still open; got %q", want, out.String())
}

// startSession runs an applet with an input the test keeps open.
func startSession(t *testing.T, name string) (io.WriteCloser, *lockedBuffer, <-chan error) {
	t.Helper()
	applet, found := DefaultRegistry.Lookup(name)
	if !found {
		t.Fatalf("%s is not registered", name)
	}
	reader, writer := io.Pipe()
	out := &lockedBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- applet.Run(context.Background(), nil, reader, out, io.Discard)
	}()
	t.Cleanup(func() {
		writer.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the applet did not finish after its input was closed")
		}
	})
	return writer, out, done
}

func TestBcAnswersBeforeTheInputEnds(t *testing.T) {
	t.Parallel()
	writer, out, _ := startSession(t, "bc")
	fmt.Fprint(writer, "2+3\n")
	waitForOutput(t, out, "5\n")
	// And it keeps the session's state, so a later line sees the earlier one's scale.
	fmt.Fprint(writer, "scale=4\n")
	fmt.Fprint(writer, "8/3\n")
	waitForOutput(t, out, "2.6666\n")
	// A definition spread over several lines is held until it closes, then runs.
	fmt.Fprint(writer, "define f(n) {\n")
	fmt.Fprint(writer, "return(n*2)\n")
	fmt.Fprint(writer, "}\n")
	fmt.Fprint(writer, "f(21)\n")
	waitForOutput(t, out, "42\n")
}

// TestBcSessionSurvivesAMistake covers the other half of reading line by line: one bad line
// does not throw away the session, which is what reading to the end first did.
func TestBcSessionSurvivesAMistake(t *testing.T) {
	t.Parallel()
	writer, out, _ := startSession(t, "bc")
	fmt.Fprint(writer, "1+1\n")
	waitForOutput(t, out, "2\n")
	fmt.Fprint(writer, "zz(\n")
	fmt.Fprint(writer, "3+3\n")
	waitForOutput(t, out, "6\n")
}

func TestDcAnswersBeforeTheInputEnds(t *testing.T) {
	t.Parallel()
	writer, out, _ := startSession(t, "dc")
	fmt.Fprint(writer, "7 8 * p\n")
	waitForOutput(t, out, "56\n")
	fmt.Fprint(writer, "2 2 + p\n")
	waitForOutput(t, out, "4\n")
	// A `[` that has not closed holds the line rather than failing, and what came before it
	// runs once and only once when it does close.
	fmt.Fprint(writer, "[9 9 *\n")
	fmt.Fprint(writer, "] x p\n")
	waitForOutput(t, out, "81\n")
}

// TestDcHoldsAnOpenBracketWithoutRepeating is the reason the bracket count is taken before
// anything runs rather than after a failed attempt.
func TestDcHoldsAnOpenBracketWithoutRepeating(t *testing.T) {
	t.Parallel()
	writer, out, _ := startSession(t, "dc")
	// The `5 p` is on the same line as the unclosed `[`. Running the line and retrying the
	// whole buffer when it closed would print 5 twice.
	fmt.Fprint(writer, "5 p [1 2\n")
	fmt.Fprint(writer, "+] x p\n")
	waitForOutput(t, out, "3\n")
	if got := strings.Count(out.String(), "5\n"); got != 1 {
		t.Fatalf("the line before the open bracket ran %d times, want once: %q", got, out.String())
	}
}
