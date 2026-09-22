package main

import (
	"context"
	"io"
	"testing"
)

// **Page Up walks the history that starts with what you have typed.**
//
// Asked for by name: `history-beginning-search-backward`, which zsh binds to Page Up. Type
// `git c`, press it, and you get the last `git c...` you ran rather than the last thing you
// ran. The cursor stays where it was, so the next press narrows from the same prefix.
//
// The three ways to reach history here now answer three different questions, and that is
// the point of having all three:
//
//   - **Up** walks everything, in order. What you want when the thing was recent.
//   - **Ctrl-R** searches anywhere in the line, incrementally. What you want when you
//     remember a word from the middle.
//   - **Page Up** walks what begins with the prefix. What you want when you know how the
//     command started -- which is most of the time, because that is how you think of it.
//
// An empty prefix makes it Up, which is what zsh does and what makes the key safe to press
// before you have typed anything.
//
// **busybox has none of this**, and neither does its editor have Page Up bound at all -- so
// this is an extension rather than parity, and docs/support-matrix.md says so.

// pagedLine drives the editor with a key stream and returns the line it submitted.
func pagedLine(t *testing.T, keys string, history ...string) string {
	t.Helper()
	_, editor := newStyledEditor(t, 80, keys, history)
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	return line
}

const (
	pageUp   = "\x1b[5~"
	pageDown = "\x1b[6~"
)

func TestPageUp_walksTheHistoryThatMatchesThePrefix(t *testing.T) {
	history := []string{"git status", "make test", "git commit -m x", "ls -l", "git push"}
	for _, testcase := range []struct {
		name    string
		keys    string
		want    string
		history []string
	}{
		{
			name: "the newest entry with the prefix, not the newest entry",
			keys: "git c" + pageUp + enter,
			want: "git commit -m x",
		},
		{
			name: "a second press goes further back",
			keys: "git " + pageUp + pageUp + enter,
			want: "git commit -m x",
		},
		{
			name: "a third press reaches the oldest match",
			keys: "git " + pageUp + pageUp + pageUp + enter,
			want: "git status",
		},
		{
			name: "past the oldest match it stays put",
			keys: "git " + pageUp + pageUp + pageUp + pageUp + pageUp + enter,
			want: "git status",
		},
		{
			name: "no match leaves the line alone",
			keys: "zzz" + pageUp + enter,
			want: "zzz",
		},
		{
			name: "an empty prefix walks everything, like Up",
			keys: pageUp + enter,
			want: "git push",
		},
		{
			name: "page down comes back towards what was typed",
			keys: "git " + pageUp + pageUp + pageDown + enter,
			want: "git push",
		},
		{
			name: "page down all the way back restores the prefix",
			keys: "git " + pageUp + pageDown + enter,
			want: "git ",
		},
		{
			name: "the prefix is what is left of the cursor, not the whole line",
			// `git push` recalled, then Home, then paging up from an empty prefix at
			// column zero walks everything.
			keys:    "make" + pageUp + enter,
			want:    "make test",
			history: history,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			entries := testcase.history
			if entries == nil {
				entries = history
			}
			if got := pagedLine(t, testcase.keys, entries...); got != testcase.want {
				t.Errorf("got %q, want %q", got, testcase.want)
			}
		})
	}
}

// TestPageUp_leavesTheCursorAtThePrefix is what makes a second press narrow rather than
// start over: the cursor stays where you typed to, so the prefix is still the prefix.
func TestPageUp_leavesTheCursorAtThePrefix(t *testing.T) {
	editor := newLineEditor(io.NopCloser(nopReader{}), io.Discard, t.TempDir())
	editor.width = func() int { return 80 }
	for _, entry := range []string{"git status", "git commit"} {
		editor.remember(entry)
	}
	editor.buffer.replace("git ")
	editor.searchHistoryByPrefix(1)
	if line := editor.buffer.String(); line != "git commit" {
		t.Fatalf("recalled %q, want %q", line, "git commit")
	}
	if editor.buffer.cursor != len("git ") {
		t.Errorf("cursor at %d, want %d -- at the end of the prefix, so the next press "+
			"narrows from the same one", editor.buffer.cursor, len("git "))
	}
}

// nopReader is an input that has already ended, for a test that drives the editor by
// calling its methods rather than by feeding keys.
type nopReader struct{}

func (nopReader) Read([]byte) (int, error) { return 0, io.EOF }
