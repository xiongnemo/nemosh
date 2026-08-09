package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func editLine(t *testing.T, keys string, history ...string) (string, error, string) {
	t.Helper()
	var screen bytes.Buffer
	editor := newLineEditor(strings.NewReader(keys), &screen, t.TempDir())
	for _, entry := range history {
		editor.remember(entry)
	}
	line, err := editor.readLine(context.Background(), "$ ")
	return line, err, screen.String()
}

func TestLineEditor_returnsTheTypedLine(t *testing.T) {
	// When
	line, err, _ := editLine(t, "echo hi\r")

	// Then
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "echo hi" {
		t.Fatalf("line = %q, want %q", line, "echo hi")
	}
}

func TestLineEditor_editsBeforeSubmitting(t *testing.T) {
	for _, test := range []struct {
		name string
		keys string
		want string
	}{
		{name: "backspace", keys: "echoX\x7f\r", want: "echo"},
		{name: "backspace over a wide character", keys: "a你\x7f\r", want: "a"},
		{name: "left then insert", keys: "ac\x1b[Db\r", want: "abc"},
		{name: "home then insert", keys: "bc\x01a\r", want: "abc"},
		{name: "end after home", keys: "ab\x01x\x05c\r", want: "xabc"},
		{name: "ctrl-u clears the line", keys: "throwaway\x15kept\r", want: "kept"},
		{name: "ctrl-w deletes a word", keys: "echo one two\x17\r", want: "echo one "},
		{name: "delete removes forward", keys: "abc\x01\x1b[3~\r", want: "bc"},
		{name: "right moves back over", keys: "ab\x01\x1b[Cx\r", want: "axb"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			line, err, _ := editLine(t, test.keys)

			// Then
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// Ctrl-D on an empty line ends input, which is what exits the shell. On a line
// with text it deletes forward instead, because ending input there would throw
// the line away.
func TestLineEditor_ctrlDEndsInputOnlyWhenTheLineIsEmpty(t *testing.T) {
	// When
	_, err, _ := editLine(t, "\x04")

	// Then
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readLine on empty Ctrl-D = %v, want io.EOF", err)
	}

	// And with text it is a forward delete
	line, err, _ := editLine(t, "abc\x01\x04\r")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "bc" {
		t.Fatalf("line = %q, want %q", line, "bc")
	}
}

// Ctrl-Z is the Windows spelling of the same thing.
func TestLineEditor_ctrlZEndsInput(t *testing.T) {
	if _, err, _ := editLine(t, "\x1a"); !errors.Is(err, io.EOF) {
		t.Fatalf("readLine on Ctrl-Z = %v, want io.EOF", err)
	}
}

func TestLineEditor_ctrlCAbandonsTheLine(t *testing.T) {
	// When
	line, err, _ := editLine(t, "half typed\x03")

	// Then
	if !errors.Is(err, errLineAbandoned) {
		t.Fatalf("readLine on Ctrl-C = %v, want errLineAbandoned", err)
	}
	if line != "" {
		t.Fatalf("line = %q, want it discarded", line)
	}
}

// History is what the arrows walk, most recent first.
func TestLineEditor_recallsHistory(t *testing.T) {
	for _, test := range []struct {
		name string
		keys string
		want string
	}{
		{name: "one up", keys: "\x1b[A\r", want: "second"},
		{name: "twice up", keys: "\x1b[A\x1b[A\r", want: "first"},
		{name: "up past the end stays", keys: "\x1b[A\x1b[A\x1b[A\r", want: "first"},
		{name: "up then down returns", keys: "\x1b[A\x1b[A\x1b[B\r", want: "second"},
		{name: "down past the newest is the empty line again", keys: "\x1b[A\x1b[B\r", want: ""},
		{name: "a recalled line can be edited", keys: "\x1b[A!\r", want: "second!"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			line, err, _ := editLine(t, test.keys, "first", "second")

			// Then
			if err != nil {
				t.Fatalf("readLine: %v", err)
			}
			if line != test.want {
				t.Fatalf("line = %q, want %q", line, test.want)
			}
		})
	}
}

// A blank line and a repeat of the previous entry are not remembered, which is
// what every shell does and what keeps the arrows useful.
func TestLineEditor_doesNotRememberBlanksOrRepeats(t *testing.T) {
	// Given
	editor := newLineEditor(strings.NewReader(""), &bytes.Buffer{}, t.TempDir())

	// When
	editor.remember("one")
	editor.remember("one")
	editor.remember("")
	editor.remember("   ")
	editor.remember("two")

	// Then
	if got := editor.entries(); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("entries = %v, want [one two]", got)
	}
}

// Tab completes a command at the start of a line.
func TestLineEditor_completesACommand(t *testing.T) {
	// When
	line, err, _ := editLine(t, "expor\t\r")

	// Then
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "export " {
		t.Fatalf("line = %q, want %q", line, "export ")
	}
}

// A unique completion appends a blank, so the next word can be typed straight
// away. An ambiguous one inserts only what the candidates share.
func TestLineEditor_insertsOnlyTheSharedPrefixWhenAmbiguous(t *testing.T) {
	// When: `re` matches readonly, readlink, realpath and return
	line, err, screen := editLine(t, "re\t\r")

	// Then
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "rea" && line != "re" {
		t.Fatalf("line = %q, want the shared prefix only", line)
	}
	if strings.HasSuffix(line, " ") {
		t.Fatalf("line = %q, want no blank after an ambiguous completion", line)
	}
	if !strings.Contains(screen, "readonly") && !strings.Contains(screen, "realpath") {
		t.Fatalf("screen did not list the candidates: %q", screen)
	}
}

func TestLineEditor_completesNothingGracefully(t *testing.T) {
	// When
	line, err, _ := editLine(t, "zzzznosuch\t\r")

	// Then
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	if line != "zzzznosuch" {
		t.Fatalf("line = %q, want it unchanged", line)
	}
}

// End of input with text already typed submits it, the way a pipe ending after
// a final line without a newline does.
func TestLineEditor_submitsAPartialLineAtEndOfStream(t *testing.T) {
	// When
	line, err, _ := editLine(t, "echo tail")

	// Then
	if line != "echo tail" {
		t.Fatalf("line = %q, want %q (err %v)", line, "echo tail", err)
	}
}

// The redraw's cursor placement is what the prompt-width bug corrupted, so it
// is pinned by the bytes actually written rather than by the width function
// alone. With a coloured `$ ` prompt the cursor must be moved two columns, not
// eleven.
func TestLineEditor_placesTheCursorPastAColouredPrompt(t *testing.T) {
	// Given
	var screen bytes.Buffer
	editor := newLineEditor(strings.NewReader("w"), &screen, t.TempDir())

	// When: one keystroke, then the stream ends and the line is submitted
	if _, err := editor.readLine(context.Background(), "\033[1;31m$\033[0m "); err != nil {
		t.Fatal(err)
	}

	// Then: the redraw moves the cursor by the prompt's drawn width plus the
	// one character typed, which is 2 + 1.
	if !strings.Contains(screen.String(), "\033[3C") {
		t.Fatalf("screen = %q, want a move to column 3 (prompt 2 + one character)", screen.String())
	}
	// The over-count this replaces would have produced 11 + 1.
	if strings.Contains(screen.String(), "\033[12C") {
		t.Fatalf("screen = %q, the prompt's escapes are being counted as columns", screen.String())
	}
}
