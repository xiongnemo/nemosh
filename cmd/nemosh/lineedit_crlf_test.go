package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

// **A carriage return and the line feed after it are one Enter.**
//
// Reported from a real terminal: after ending `wc` or `cat` with Ctrl-Z, three prompts
// appeared in a row instead of one.
//
// In raw mode Enter arrives as a bare CR, and that is what the editor normally sees. But
// the editor is not the only thing that reads this console: a command reads it in cooked
// mode, and what a cooked console hands back ends in CRLF. When such a command ends, what
// it did not consume is still buffered, and the editor picks it up on the next line read --
// decoding CRLF as two Enters, which is two empty commands, which is two spare prompts.
//
// A bare LF stays an Enter. A stream that has been through a pipe or a file arrives that
// way, and `nemosh < script` depends on it.

func editedLines(t *testing.T, feed string) []string {
	t.Helper()
	_, editor := newStyledEditor(t, 80, feed, nil)
	var lines []string
	for range 8 {
		line, err := editor.readLine(context.Background(), "$ ")
		if err != nil {
			break
		}
		lines = append(lines, line)
	}
	return lines
}

func TestLineEditor_carriageReturnAndLineFeedAreOneEnter(t *testing.T) {
	for _, testcase := range []struct {
		name string
		feed string
		want []string
	}{
		{name: "CRLF after text", feed: "a\r\n", want: []string{"a"}},
		{name: "CRLF alone", feed: "\r\n", want: []string{""}},
		{name: "two CRLFs", feed: "\r\n\r\n", want: []string{"", ""}},
		{name: "bare CR is Enter", feed: "a\r", want: []string{"a"}},
		{name: "bare LF is Enter", feed: "a\n", want: []string{"a"}},
		{name: "LF then CR is two", feed: "\n\r", want: []string{"", ""}},
		{name: "what Ctrl-Z leaves", feed: "\r\nsecond\r\n", want: []string{"", "second"}},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			got := editedLines(t, testcase.feed)
			if len(got) != len(testcase.want) || strings.Join(got, "\x00") != strings.Join(testcase.want, "\x00") {
				t.Errorf("%q produced %d line(s) %q, want %d %q -- a spare line here is a spare "+
					"prompt on the screen", testcase.feed, len(got), got, len(testcase.want), testcase.want)
			}
		})
	}
}

// TestLineEditor_carriageReturnSplitAcrossReadsIsStillOneEnter covers the CR and the LF
// arriving in different reads, which a 64-byte read boundary or a slow paste can do. The
// editor accumulates into a pending buffer, so the two halves are not always decoded
// together and the rule cannot live only in decodeKey.
func TestLineEditor_carriageReturnSplitAcrossReadsIsStillOneEnter(t *testing.T) {
	editor := newLineEditor(&splitReader{chunks: []string{"a\r", "\nb\r\n"}}, io.Discard, t.TempDir())
	editor.width = func() int { return 80 }

	var lines []string
	for range 4 {
		line, err := editor.readLine(context.Background(), "$ ")
		if err != nil {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Errorf("split CRLF produced %q, want [a b]: the line feed that arrived in the next "+
			"read belongs to the Enter before it", lines)
	}
}

// splitReader hands back one chunk per Read, so the CR and the LF land in separate calls.
type splitReader struct{ chunks []string }

func (r *splitReader) Read(buffer []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(buffer, r.chunks[0])
	r.chunks = r.chunks[1:]
	return n, nil
}
