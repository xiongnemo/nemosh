package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// editLineWith is editLine with the shell's variables the editor reads its settings from.
func editLineWith(t *testing.T, keys string, settings map[string]string) (string, error) {
	t.Helper()
	editor := newLineEditor(strings.NewReader(keys), &bytes.Buffer{}, t.TempDir())
	editor.setting = func(name string) (string, bool) { value, ok := settings[name]; return value, ok }
	return editor.readLine(context.Background(), "$ ")
}

// Ctrl-Z turns every backslash on the line into a slash, as it does in busybox-w32's editor
// (libbb/lineedit.c, CTRL('Z')), so a Windows path pasted in -- C:\Users\nemo -- becomes one
// the shell reads unquoted. It ended input instead, so a stray Ctrl-Z, which is suspend to
// anyone used to a Unix shell, closed the session. It changes the line and nothing else: the
// cursor stays, and an empty line stays empty.
func TestLineEditor_ctrlZTurnsBackslashesIntoSlashes(t *testing.T) {
	for _, test := range []struct{ name, keys, want string }{
		{"a pasted path", `cd C:\Users\nemo` + "\x1a\r", "cd C:/Users/nemo"},
		{"the cursor stays where it was", `ls a\b` + "\x02\x02\x1aX\r", "ls aX/b"},
		{"an empty line is left alone", "\x1a\r", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			line, err := editLineWith(t, test.keys, nil)
			if err != nil || line != test.want {
				t.Fatalf("line = %q, %v, want %q", line, err, test.want)
			}
		})
	}
}

// NEMOSH_CTRL_Z=eof is the way back: Ctrl-Z is end of input on an empty line, as Ctrl-D is,
// and a forward delete on a line with text, which is what it was until 2026-11-15. Read at
// each press, so setting it in an rc file, exported or not, takes effect at once.
func TestLineEditor_ctrlZEndsInputWhenAskedTo(t *testing.T) {
	eof := map[string]string{"NEMOSH_CTRL_Z": "eof"}
	if _, err := editLineWith(t, "\x1a", eof); !errors.Is(err, io.EOF) {
		t.Fatalf("readLine on Ctrl-Z = %v, want io.EOF", err)
	}
	if line, err := editLineWith(t, `a\b`+"\x02\x02\x1a\r", eof); err != nil || line != "ab" {
		t.Fatalf("line = %q, %v, want the backslash deleted", line, err)
	}
}
