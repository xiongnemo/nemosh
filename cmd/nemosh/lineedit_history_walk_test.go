package main

import (
	"testing"
)

// **A history walk starts from what you typed, and ends by giving it back.**
//
// Reported with a key log from a real terminal, and the log is the test:
//
//	\x1b[A     Up       -> "git push"
//	\x1b[5~    Page Up  -> "git push"   nothing happened
//	\x1b[6~    Page Down-> "git push"   nothing happened
//	exit                -> "git pushexit", which then ran
//
// Every key arrived and decoded correctly, which ruled the terminal out. Two faults in the
// design, both from computing the prefix afresh from the buffer on every press:
//
//   - After Up the buffer holds a whole recalled line with the cursor at its end, so "the
//     text left of the cursor" was the entire `git push` -- and nothing older starts with
//     that. Page Up did nothing, visibly.
//   - Coming back to the line being typed put that "prefix" back on it. But the prefix had
//     come from a history entry, not from anything typed, so recalled text was left on the
//     line looking like input. What was typed next was glued to it and run.
//
// The fix is one idea: **the line being typed is saved when a walk leaves it, and handed
// back unchanged when a walk returns to it.** The prefix Page Up searches for is taken from
// that saved line, so it is always something typed and never something recalled.
//
// Down had the same fault for longer: returning to the typed line replaced it with an empty
// one, so a half-typed command was lost by pressing Up and then Down. That shares the
// mechanism, so it shares the fix.

// walkedLine drives the editor through a key stream and returns the line it submitted.
func walkedLine(t *testing.T, keys string, history ...string) string {
	t.Helper()
	return pagedLine(t, keys, history...)
}

const (
	arrowUp   = "\x1b[A"
	arrowDown = "\x1b[B"
)

func TestHistoryWalk_givesTheTypedLineBack(t *testing.T) {
	history := []string{"make test", "git status", "git push"}
	for _, testcase := range []struct {
		name string
		keys string
		want string
	}{
		{
			// The reported sequence. After it the line must hold only what was typed --
			// here, nothing -- so `exit` is `exit` and not `git pushexit`.
			name: "the key log from the terminal",
			keys: arrowUp + pageUp + pageDown + pageDown + "exit" + enter,
			want: "exit",
		},
		{
			name: "up then page up keeps walking rather than stopping",
			keys: arrowUp + pageUp + enter,
			want: "git status",
		},
		{
			name: "up then down gives a half-typed line back",
			keys: "echo hal" + arrowUp + arrowDown + "f" + enter,
			want: "echo half",
		},
		{
			name: "page up then page down gives the whole typed line back, not just the prefix",
			keys: "git s" + pageUp + pageDown + "tatus -s" + enter,
			want: "git status -s",
		},
		{
			name: "the prefix is what was typed, even after walking",
			keys: "git " + pageUp + pageUp + enter,
			want: "git status",
		},
		{
			name: "an empty prefix leaves the cursor at the end, the way Up does",
			keys: pageUp + " --dry-run" + enter,
			want: "git push --dry-run",
		},
		{
			name: "a prefix leaves the cursor after the prefix",
			keys: "git s" + pageUp + "X" + enter,
			want: "git sXtatus",
		},
		{
			name: "down past the newest entry is still the typed line",
			keys: "make" + arrowUp + arrowDown + arrowDown + enter,
			want: "make",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := walkedLine(t, testcase.keys, history...); got != testcase.want {
				t.Errorf("got %q, want %q", got, testcase.want)
			}
		})
	}
}
