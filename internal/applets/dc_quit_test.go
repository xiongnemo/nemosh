package applets

import (
	"context"
	"io"
	"testing"
	"time"
)

// **`q` leaves dc without asking for another line.**
//
// Reported from a real terminal: after `q` and Enter, dc sat there and took one more line
// before it went. The session loop said
//
//	for reader.Scan() && !m.quit {
//
// and Go evaluates the left operand first -- so `q` set the flag, and the loop then read
// another line before looking at it. At a terminal that read is a wait for a keystroke, so
// dc appeared not to have quit at all. bc has the same shape written the other way round,
// with the check inside the body, which is why it did not have this.
//
// The input is held open on purpose. Every other dc test feeds a string that has already
// ended, and a scanner over one of those answers false straight away -- so none of them
// could tell a loop that asks for another line from one that does not.
func TestDcQuitsWithoutReadingAnotherLine(t *testing.T) {
	applet, found := DefaultRegistry.Lookup("dc")
	if !found {
		t.Fatal("dc is not registered")
	}
	reader, writer := io.Pipe()
	t.Cleanup(func() { writer.Close() })
	out := &lockedBuffer{}

	done := make(chan error, 1)
	go func() { done <- applet.Run(context.Background(), nil, reader, out, io.Discard) }()

	if _, err := io.WriteString(writer, "5 3 + p\nq\n"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("dc returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dc is still reading after `q`, with its input still open: at a terminal that " +
			"is a prompt that will not come back until another line is typed")
	}
	if got := out.String(); got != "8\n" {
		t.Errorf("stdout = %q, want %q", got, "8\n")
	}
}
