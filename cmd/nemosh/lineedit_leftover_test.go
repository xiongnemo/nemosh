package main

import (
	"context"
	"io"
	"testing"
)

// **The line terminator a command left behind is not a keypress.**
//
// Reported from a real terminal: ending `wc` or `cat` with Ctrl-Z left a spare prompt.
// Measured with a key log rather than guessed at, because two earlier guesses were wrong:
//
//	[keylog] read 1 byte(s) "\r"      <- Enter, pressed
//	...wc runs, reads the console in cooked mode, ends on Ctrl-Z...
//	[keylog] read 2 byte(s) "\r\n"    <- nobody pressed anything
//
// A console read in cooked mode hands back a line ending CRLF, and the terminator of the
// line the command consumed is still there when the editor reads next. It is not visible
// to GetNumberOfConsoleInputEvents -- that counts input records, and this is in the line
// buffer, which is why probing the console for it found nothing twice.
//
// The discriminator is the pair. In raw mode, which is the only mode the editor reads in,
// Enter arrives as a bare CR: one byte. Two bytes is not something a keyboard produces
// here. Paired with "a command has just run", which the session knows, that is narrow
// enough to drop without ever dropping a real keystroke -- and a paste, whose first key is
// a character rather than a newline, clears the flag untouched.

func TestLineEditor_dropsTheTerminatorACommandLeftBehind(t *testing.T) {
	for _, testcase := range []struct {
		name     string
		feed     string
		leftover bool
		want     []string
	}{
		{
			name:     "what Ctrl-Z leaves is dropped",
			feed:     "\r\nafter\r",
			leftover: true,
			want:     []string{"after"},
		},
		{
			name:     "a real Enter is a bare CR and is kept",
			feed:     "\rafter\r",
			leftover: true,
			want:     []string{"", "after"},
		},
		{
			name:     "only the first key is eligible",
			feed:     "first\r\r\nsecond\r",
			leftover: true,
			want:     []string{"first", "", "second"},
		},
		{
			name:     "a paste starting with text keeps its newlines",
			feed:     "one\r\ntwo\r\n",
			leftover: true,
			want:     []string{"one", "two"},
		},
		{
			name:     "without a command having run, nothing is dropped",
			feed:     "\r\nafter\r",
			leftover: false,
			want:     []string{"", "after"},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			_, editor := newStyledEditor(t, 80, testcase.feed, nil)
			// Set once, before the first line: the flag describes the read that follows
			// one command, which is what the session sets it for.
			editor.afterCookedCommand = testcase.leftover
			var lines []string
			for range 8 {
				line, err := editor.readLine(context.Background(), "$ ")
				if err != nil {
					break
				}
				lines = append(lines, line)
			}
			if len(lines) != len(testcase.want) {
				t.Fatalf("%q produced %d line(s) %q, want %d %q",
					testcase.feed, len(lines), lines, len(testcase.want), testcase.want)
			}
			for index := range lines {
				if lines[index] != testcase.want[index] {
					t.Errorf("line %d = %q, want %q", index, lines[index], testcase.want[index])
				}
			}
		})
	}
}

// TestLineEditor_leftoverFlagIsSpentOnOneLine keeps the flag from eating a later Enter:
// it describes the read that follows one command, and nothing after that.
func TestLineEditor_leftoverFlagIsSpentOnOneLine(t *testing.T) {
	// Three terminators: the first is the leftover and is dropped, and the two after it are
	// ordinary empty lines. A flag that stayed set would swallow the third as well.
	editor := newLineEditor(&splitReader{chunks: []string{"\r\n", "\r\n", "\r\n"}}, io.Discard, t.TempDir())
	editor.width = func() int { return 80 }
	editor.afterCookedCommand = true

	first, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("first line: %v", err)
	}
	second, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("second line: %v", err)
	}
	// The first read swallowed the leftover and then took the next Enter as the line; the
	// second is an ordinary empty line. Either way there must be exactly two, because a
	// flag that stayed set would silently eat every blank line from here on.
	if first != "" || second != "" {
		t.Errorf("got %q and %q, want two empty lines", first, second)
	}
}
