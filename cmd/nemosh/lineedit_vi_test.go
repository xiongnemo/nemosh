package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// viEditor is an editor in vi mode reading keys, where another key is waiting whenever the
// reader has more.
func viEditor(t *testing.T, input io.Reader, waiting func() bool, history ...string) *lineEditor {
	t.Helper()
	_, editor := newStyledEditor(t, 80, "", history)
	editor.input = input
	editor.inputWaiting = func(time.Duration) bool { return waiting() }
	editor.vi.on = true
	return editor
}

func viLine(t *testing.T, keys string, history ...string) string {
	t.Helper()
	reader := strings.NewReader(keys)
	line, err := viEditor(t, reader, func() bool { return reader.Len() > 0 }, history...).readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return line
}

// The answers are busybox's, read from libbb/lineedit.c: its key loop's VI_CMDMODE_BIT cases
// and vi_word_motion and its siblings.
func TestLineEditor_viCommandMode(t *testing.T) {
	const escape = "\x1b"
	for _, test := range []struct {
		name, keys, want string
		history          []string
	}{
		{name: "Escape goes back one, and a letter there is a command", keys: "abc" + escape + "x\r", want: "ab"},
		{name: "a second Escape stays where it is", keys: "abc" + escape + escape + "x\r", want: "ab"},
		{name: "a letter that is no command is not put in", keys: "abc" + escape + "zq\r", want: "abc"},
		{name: "Backspace goes left", keys: "abc" + escape + "\x7fx\r", want: "ac"},
		{name: "i inserts at the cursor", keys: "abc" + escape + "iX\r", want: "abXc"},
		{name: "a inserts after it", keys: "abc" + escape + "aX\r", want: "abcX"},
		{name: "I and A insert at the ends", keys: "abc" + escape + "IX" + escape + "AY\r", want: "XabcY"},
		{name: "0 is the start", keys: "abc" + escape + "0x\r", want: "bc"},
		{name: "$ is past the end", keys: "abc" + escape + "0$iX\r", want: "abcX"},
		{name: "h and l", keys: "abc" + escape + "hx0lx\r", want: "a"},
		{name: "X deletes before the cursor", keys: "abc" + escape + "X\r", want: "ac"},
		{name: "w goes to the next word", keys: "one two" + escape + "0wx\r", want: "one wo"},
		{name: "w stops at punctuation", keys: "a.b c" + escape + "0wx\r", want: "ab c"},
		{name: "W goes past it", keys: "a.b c" + escape + "0Wx\r", want: "a.b "},
		{name: "e goes to the end of the word", keys: "one two" + escape + "0ex\r", want: "on two"},
		{name: "E to the end of the blank-separated one", keys: "a.b c" + escape + "0Ex\r", want: "a. c"},
		{name: "b goes back a word", keys: "one two" + escape + "bx\r", want: "one wo"},
		{name: "B goes back over punctuation", keys: "x a.b" + escape + "Bx\r", want: "x .b"},
		{name: "dw takes the blanks after the word", keys: "one two" + escape + "0dw\r", want: "two"},
		{name: "cw leaves them, and inserts", keys: "one two" + escape + "0cwX\r", want: "X two"},
		{name: "de takes the word's last character", keys: "one two" + escape + "0de\r", want: " two"},
		{name: "db deletes back to the word's start", keys: "one two" + escape + "db\r", want: "one o"},
		{name: "dd is the whole line", keys: "one two" + escape + "dd\r", want: ""},
		{name: "cc is the whole line, and inserts", keys: "one" + escape + "ccX\r", want: "X"},
		{name: "d$ deletes to the end", keys: "one two" + escape + "0wd$\r", want: "one "},
		{name: "D deletes to the end", keys: "one two" + escape + "0wD\r", want: "one "},
		{name: "C deletes to the end, and inserts", keys: "one two" + escape + "0wCX\r", want: "one X"},
		{name: "d and a space deletes one", keys: "abc" + escape + "0d \r", want: "bc"},
		{name: "d takes a key it has no use for, and does nothing", keys: "abc" + escape + "dxx\r", want: "ab"},
		{name: "p puts the deletion after the cursor", keys: "abc" + escape + "0xp\r", want: "bac"},
		{name: "P puts it at the cursor", keys: "abc" + escape + "x0P\r", want: "cab"},
		{name: "what dw took is put whole", keys: "one two" + escape + "0dw$p\r", want: "twoone "},
		{name: "r replaces the character under the cursor", keys: "abc" + escape + "0rX\r", want: "Xbc"},
		{name: "r past the end changes nothing", keys: "abc" + escape + "$rX\r", want: "abc"},
		{name: "k is the line before, from its start", keys: escape + "kx\r", want: "cho one", history: []string{"echo one"}},
		{name: "Up in insert mode starts there too", keys: "\x1b[AX\r", want: "Xecho one", history: []string{"echo one"}},
		{name: "j comes back to the line typed", keys: "ab" + escape + "kjx\r", want: "b", history: []string{"one"}},
		{name: "an Escape with [ after it is a sequence", keys: "abc\x1b[DX\r", want: "abXc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := viLine(t, test.keys, test.history...); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// chunkedInput hands over its chunks a read at a time, as a terminal does keys typed apart.
type chunkedInput struct{ chunks []string }

func (c *chunkedInput) Read(buffer []byte) (int, error) {
	if len(c.chunks) == 0 {
		return 0, io.EOF
	}
	count := copy(buffer, c.chunks[0])
	c.chunks = c.chunks[1:]
	return count, nil
}

// An Escape is the start of a sequence when the rest of one comes in time, and the key on its
// own when nothing does.
func TestLineEditor_viEscapeWaitsForTheRestOfASequence(t *testing.T) {
	for _, test := range []struct {
		name, want string
		waiting    bool
	}{
		{name: "the rest comes in time", waiting: true, want: "abXc"},
		{name: "nothing comes", waiting: false, want: "ab"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			input := &chunkedInput{chunks: []string{"abc\x1b", "[DX\r"}}
			if !test.waiting {
				input.chunks[1] = "x\r"
			}
			editor := viEditor(t, input, func() bool { return test.waiting && len(input.chunks) > 0 })
			line, err := editor.readLine(context.Background(), "$ ")

			// Then
			if err != nil || line != test.want {
				t.Fatalf("got %q, %v, want %q", line, err, test.want)
			}
		})
	}
}

// busybox starts each line in insert mode, with nothing deleted to put back.
func TestLineEditor_viStartsEachLineAfresh(t *testing.T) {
	// When
	reader := strings.NewReader("abc\x1bx\r\x1bP\rxy\r")
	editor := viEditor(t, reader, func() bool { return reader.Len() > 0 })
	var lines []string
	for range 3 {
		line, err := editor.readLine(context.Background(), "$ ")
		if err != nil {
			t.Fatalf("readLine: %v", err)
		}
		lines = append(lines, line)
	}

	// Then
	if got := strings.Join(lines, "|"); got != "ab||xy" {
		t.Fatalf("lines = %q, want %q", got, "ab||xy")
	}
}
