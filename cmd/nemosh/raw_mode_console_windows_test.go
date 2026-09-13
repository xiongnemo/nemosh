package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// What raw mode actually does to a Windows console, and that giving it back gives back
// exactly what was there.
//
// The bug this guards was not in the mode-setting -- that was always right -- but in *how
// long* it was held. Still, the two halves are worth pinning: that raw mode really does
// clear the three bits a command needs, which is why holding it broke `bc`; and that restore
// puts the mode back bit for bit, which is what makes borrowing it per line read safe to do
// a hundred times a session.
//
// A real console, because there is nothing else to ask. The test skips where there is none,
// which is every CI runner.

func openTestConsole(t *testing.T) *os.File {
	t.Helper()
	console, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no console to test against: %v", err)
	}
	t.Cleanup(func() { console.Close() })
	return console
}

func consoleMode(t *testing.T, console *os.File) uint32 {
	t.Helper()
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(console.Fd()), &mode); err != nil {
		t.Skipf("this handle is not a console: %v", err)
	}
	return mode
}

func TestRawMode_clearsWhatACommandNeedsAndGivesItBack(t *testing.T) {
	console := openTestConsole(t)
	before := consoleMode(t, console)

	raw := enterRawMode(console)
	if raw == nil {
		t.Skip("this console will not go raw")
	}
	// Restored no matter how this test leaves, including a panic: the console is shared
	// with whatever started the test, and handing it back raw is the failure being
	// guarded against.
	defer raw.restore()

	during := consoleMode(t, console)
	for _, bit := range []struct {
		name string
		mask uint32
	}{
		{"ENABLE_ECHO_INPUT", windows.ENABLE_ECHO_INPUT},
		{"ENABLE_LINE_INPUT", windows.ENABLE_LINE_INPUT},
		{"ENABLE_PROCESSED_INPUT", windows.ENABLE_PROCESSED_INPUT},
	} {
		if before&bit.mask == 0 {
			// The console did not have it to begin with, so its absence proves nothing.
			continue
		}
		if during&bit.mask != 0 {
			t.Errorf("raw mode left %s set", bit.name)
		}
	}
	// The third of those is the one that makes Ctrl-C an interrupt rather than a byte,
	// which is why a command must never run while this mode is in force.
	if before&windows.ENABLE_PROCESSED_INPUT != 0 && during&windows.ENABLE_PROCESSED_INPUT != 0 {
		t.Error("raw mode kept ENABLE_PROCESSED_INPUT, so this test cannot show the difference")
	}

	raw.restore()
	if after := consoleMode(t, console); after != before {
		t.Fatalf("the console came back as 0x%04x, not the 0x%04x it was lent as", after, before)
	}
}

// TestReadLineInRawMode_givesTheTerminalBack is the invariant as behaviour rather than as
// source: whatever the editor did, the console is cooked again by the time the session gets
// on with running the command that was typed.
//
// The editor reads from a reader that has already ended, so readLine returns at once and the
// console is never actually read from -- the test is about the terminal's state, not about
// editing.
func TestReadLineInRawMode_givesTheTerminalBack(t *testing.T) {
	console := openTestConsole(t)
	before := consoleMode(t, console)

	editor := newLineEditor(strings.NewReader(""), io.Discard, ".")
	if _, err := readLineInRawMode(context.Background(), console, editor, ""); err == nil {
		t.Fatal("the editor read a line from an empty reader")
	}

	after := consoleMode(t, console)
	if after != before {
		t.Fatalf("the console was left as 0x%04x, not the 0x%04x it was lent as", after, before)
	}
	// And named explicitly, because this bit is the one that decides whether Ctrl-C during
	// the next command is an interrupt or a byte nobody reads.
	if before&windows.ENABLE_PROCESSED_INPUT != 0 && after&windows.ENABLE_PROCESSED_INPUT == 0 {
		t.Fatal("ENABLE_PROCESSED_INPUT was not given back, so Ctrl-C would do nothing")
	}
}
