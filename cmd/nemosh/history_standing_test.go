package main

import (
	"slices"
	"testing"
)

// `!!` was drawn red, which looked like a mistake every time it was typed.
//
// The colour asks "can this shell run a command of this name?", and `!!` is not a command
// name -- it is a textual rewrite that happens before the parser sees anything, so no
// amount of looking in PATH for a program called `!!` will find one. Answering
// "undetermined" would fix the red and throw away the useful part: the editor knows what
// `!!` will become, so it can colour it by what will actually run.

// standingEditor builds an editor whose PATH index is **ready** and holds nothing.
//
// Readiness is the point: an unseeded index answers `undetermined` for every name, which
// is the honest colour before PATH has been read but makes every case here look the same.
// The first version of this test used one and reported `undetermined` for all eight words,
// which said nothing about the change at all.
func standingEditor(t *testing.T, history []string, runnable ...string) *lineEditor {
	t.Helper()
	index := newPathIndex()
	index.mu.Lock()
	index.ready, index.builtOn = true, "seeded-by-test"
	index.mu.Unlock()

	editor := &lineEditor{commands: newShellCommands(index), history: history}
	editor.commands.set(runnable)
	return editor
}

func TestCommandStanding_historyReference(t *testing.T) {
	history := []string{"ls -la /tmp", "nosuchprogramxyz arg"}
	for _, test := range []struct {
		name string
		word string
		want commandStanding
	}{
		// `!!` is the previous line, so it is judged by that line's command. Here the
		// previous line cannot run, so red is still right -- but for the honest reason.
		{name: "!! of an unrunnable command", word: "!!", want: standingUnknown},
		// A prefix reference reaching a runnable command is green.
		{name: "!ls resolves to ls", word: "!ls", want: standingRunnable},
		{name: "!-2 resolves to ls", word: "!-2", want: standingRunnable},
		{name: "!1 resolves to ls", word: "!1", want: standingRunnable},
		// An event that does not exist will be refused rather than run.
		{name: "an event that is not there", word: "!zzznotthere", want: standingUnknown},

		// Words that merely contain the characters are judged as typed, because a `!`
		// that begins nothing is ordinary text.
		{name: "a plain runnable name", word: "ls", want: standingRunnable},
		{name: "a plain unknown name", word: "nosuchprogramxyz", want: standingUnknown},
		{name: "not-equal is not a reference", word: "x!=y", want: standingUnknown},
		{name: "a trailing bang", word: "ls!", want: standingUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := standingEditor(t, history, "ls", "echo")
			if got := editor.commandStanding(test.word); got != test.want {
				t.Fatalf("commandStanding(%q) = %v, want %v", test.word, got, test.want)
			}
		})
	}
}

// `!!` after a runnable command is green, which is the case the whole change exists for.
func TestCommandStanding_bangBangGoesGreen(t *testing.T) {
	editor := standingEditor(t, []string{"ls -la /tmp"}, "ls")
	if got := editor.commandStanding("!!"); got != standingRunnable {
		t.Fatalf("`!!` after `ls -la` is %v, want runnable -- it was drawn red before", got)
	}
	// And `^old^new`, which repeats the previous line with a substitution, is judged the
	// same way.
	if got := editor.commandStanding("^tmp^var"); got != standingRunnable {
		t.Fatalf("`^tmp^var` after `ls -la /tmp` is %v, want runnable", got)
	}
}

// With no history at all, a reference resolves to nothing and stays red rather than
// claiming it will run.
func TestCommandStanding_noHistory(t *testing.T) {
	editor := standingEditor(t, nil, "ls")
	if got := editor.commandStanding("!!"); got != standingUnknown {
		t.Fatalf("`!!` with no history is %v, want unknown", got)
	}
	// A name that happens to contain `!` is still judged as itself.
	if got := editor.commandStanding("ls"); got != standingRunnable {
		t.Fatalf("`ls` with no history is %v, want runnable", got)
	}
}

func TestFirstWordOf(t *testing.T) {
	for _, test := range []struct{ line, want string }{
		{line: "ls -la", want: "ls"},
		{line: "  ls -la", want: "ls"},
		{line: "ls", want: "ls"},
		{line: "", want: ""},
		{line: "ls\t-la", want: "ls"},
		// A pipeline's first word is the command being coloured.
		{line: "grep x | wc -l", want: "grep"},
	} {
		t.Run(test.line, func(t *testing.T) {
			if got := firstWordOf(test.line); got != test.want {
				t.Fatalf("firstWordOf(%q) = %q, want %q", test.line, got, test.want)
			}
		})
	}
}

// The end of the chain, asserted on the codes the span actually carries -- because "it
// was red" is what was reported, and green against red is the whole fix.
//
// Asserted through highlight() rather than on paint()'s output text: the escape sequence
// combines parameters, so a word under the cursor renders `[32;4m` and a test looking
// for `[32m` finds nothing and reports neither colour. That is what the first version
// of this test did.
func TestPaint_historyReferenceIsGreen(t *testing.T) {
	codesFor := func(editor *lineEditor, line string) []string {
		spans := highlight(line, len(line), defaultPalette(), editor.commandStanding)
		if len(spans) == 0 {
			t.Fatalf("%q produced no spans", line)
		}
		return spans[0].codes
	}
	const green, red = "32", "31"

	runnable := standingEditor(t, []string{"ls -la /tmp"}, "ls")
	if got := codesFor(runnable, "!!"); !slices.Contains(got, green) {
		t.Errorf("`!!` after a runnable command has codes %v, want green %q", got, green)
	}
	if got := codesFor(runnable, "!!"); slices.Contains(got, red) {
		t.Errorf("`!!` after a runnable command is still red: %v", got)
	}

	// A reference that will not run keeps its red, which is the honest answer rather
	// than a blanket exemption for anything beginning with `!`.
	stale := standingEditor(t, []string{"nosuchprogramxyz arg"}, "ls")
	if got := codesFor(stale, "!!"); !slices.Contains(got, red) {
		t.Errorf("`!!` of an unrunnable command has codes %v, want red %q", got, red)
	}

	// And the plain cases still behave, so the change did not colour everything green.
	if got := codesFor(runnable, "ls"); !slices.Contains(got, green) {
		t.Errorf("`ls` has codes %v, want green", got)
	}
	if got := codesFor(runnable, "nosuchprogramxyz"); !slices.Contains(got, red) {
		t.Errorf("`nosuchprogramxyz` has codes %v, want red", got)
	}
}
