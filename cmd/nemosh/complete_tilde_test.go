package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// `~/` has to complete, and what it inserts has to stay spelled `~/`.
//
// It offered nothing at all: completePaths split the raw word, joined `~/` onto the working
// directory, read a directory ending in a literal tilde, failed, and returned nil. Silently, so Tab
// looked like a dead key rather than a failure -- the same shape of silence that hid `top -H` and
// the stdin lease.
//
// Expansion happens during *word* expansion, in Runtime.expandHomeTilde, and completion works on the
// text as typed, so there was no route to it. The home directory is passed in here for the same
// reason the working directory already is: the editor caches both from the runtime rather than
// reaching for a Runtime it does not hold.

// tildeHome makes a home directory with known contents and returns it.
func tildeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	for _, name := range []string{"Documents", "Downloads"} {
		if err := os.Mkdir(filepath.Join(home, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"notes.txt", "profile"} {
		if err := os.WriteFile(filepath.Join(home, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestCompletePaths_completesUnderTheHomeDirectory(t *testing.T) {
	home := tildeHome(t)
	// A working directory with none of those names in it, so anything offered came from home.
	elsewhere := t.TempDir()

	tests := []struct {
		name   string
		prefix string
		want   []string
	}{
		{
			name: "the bare home directory", prefix: "~/",
			want: []string{"~/Documents/", "~/Downloads/", "~/notes.txt", "~/profile"},
		},
		{
			// The spelling is kept. Expanding it in the buffer would rewrite `~/Do`
			// into a full absolute path, which bash does not do and which turns a
			// short line into a long one on a keypress.
			name: "a stem under it", prefix: "~/Do",
			want: []string{"~/Documents/", "~/Downloads/"},
		},
		{
			name: "a file under it", prefix: "~/no",
			want: []string{"~/notes.txt"},
		},
		{
			// `~` alone completes to the home directory itself, so a second Tab can
			// descend rather than the key doing nothing.
			name: "the tilde alone", prefix: "~",
			want: []string{"~/"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := completePathsFrom(elsewhere, home, test.prefix, false)
			if !slices.Equal(got, test.want) {
				t.Fatalf("completePaths(%q) = %v, want %v", test.prefix, got, test.want)
			}
		})
	}
}

// Directories only, where the command takes one: the existing per-command rule has to keep working
// through the tilde.
func TestCompletePaths_tildeHonoursDirectoriesOnly(t *testing.T) {
	home := tildeHome(t)

	got := completePathsFrom(t.TempDir(), home, "~/", true)

	want := []string{"~/Documents/", "~/Downloads/"}
	if !slices.Equal(got, want) {
		t.Fatalf("directories under ~ = %v, want %v", got, want)
	}
}

// `~user` is not resolved, and must not be guessed at. `echo ~root` prints `~root` today and
// docs/design/v1-scope.md defers another account's profile directory, so offering anything here
// would be inventing an answer.
func TestCompletePaths_anotherUsersTildeOffersNothing(t *testing.T) {
	home := tildeHome(t)

	if got := completePathsFrom(t.TempDir(), home, "~root/", false); len(got) != 0 {
		t.Fatalf("~root/ offered %v, want nothing", got)
	}
	if got := completePathsFrom(t.TempDir(), home, "~root", false); len(got) != 0 {
		t.Fatalf("~root offered %v, want nothing", got)
	}
}

// With no home known, a tilde is left alone rather than completed against the working directory --
// which would silently offer the wrong directory's contents under a `~/` spelling.
func TestCompletePaths_noHomeMeansNoTildeCompletion(t *testing.T) {
	if got := completePathsFrom(tildeHome(t), "", "~/", false); len(got) != 0 {
		t.Fatalf("~/ with no home offered %v, want nothing", got)
	}
}

// And an ordinary relative prefix still works, so the tilde rule cannot have swallowed the
// common case.
func TestCompletePaths_ordinaryPrefixesStillWork(t *testing.T) {
	directory := tildeHome(t)

	got := completePathsFrom(directory, "/somewhere/else", "no", false)

	if !slices.Equal(got, []string{"notes.txt"}) {
		t.Fatalf("relative completion = %v, want [notes.txt]", got)
	}
}

// The tilde a completion produced must survive insertion unescaped.
//
// This nearly undid the whole fix. `shellSpecialCharacters` includes `~`, taken from busybox's
// is_special_char, and busybox is right to escape it: a tilde in the *middle* of a filename is
// literal and needs protecting. But the leading `~/` here is not part of a name -- it is the home
// directory reference this completion deliberately produced -- and escaping it inserts `\~/`, which
// the shell then treats as a literal directory called `~`. Tab would appear to work and produce a
// line that cannot run.
func TestEscapeForInsertion_keepsALeadingTildeUsable(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "a home directory reference", text: "~/", want: "~/"},
		{name: "a path under it", text: "~/Documents/", want: "~/Documents/"},
		{
			// The blank still has to be escaped, or the line names two operands.
			name: "with a blank under it", text: "~/My Documents/", want: `~/My\ Documents/`,
		},
		{
			// A tilde anywhere else is part of a filename and keeps its backslash,
			// which is the case busybox's list exists for.
			name: "a tilde inside a name", text: "back~up.txt", want: `back\~up.txt`,
		},
		{
			name: "a tilde inside a directory", text: "a/~b/", want: `a/\~b/`,
		},
		{
			// A relative path that merely starts with a tilde-named file is still a
			// filename: `~foo` is not a home reference this shell resolves.
			name: "another user's tilde", text: "~root/", want: `\~root/`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := escapeForInsertion(test.text); got != test.want {
				t.Fatalf("escapeForInsertion(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

// And what was typed is matched against real names, so the unescaping has to agree.
func TestUnescapeTypedWord_leavesAnUnescapedTildeAlone(t *testing.T) {
	for _, text := range []string{"~/", "~/Doc", `~/My\ Doc`} {
		got := unescapeTypedWord(text)
		want := strings.ReplaceAll(text, `\ `, " ")
		if got != want {
			t.Fatalf("unescapeTypedWord(%q) = %q, want %q", text, got, want)
		}
	}
}

// The whole thing from the keyboard: type `ls ~/Doc`, press Tab, and read the line back.
//
// Through the editor because that is where the two halves meet. The completion produces `~/Documents/`
// and the insertion escapes what it is given, and each was correct on its own while the pair produced
// `\~/Documents/` -- a line that looks completed and cannot run.
func TestLineEditor_completesUnderTheTilde(t *testing.T) {
	home := tildeHome(t)
	screen := newScreenModel(t, 70)
	editor := newLineEditor(strings.NewReader("ls ~/Docu\t\r"), screen, t.TempDir())
	editor.width = func() int { return 70 }
	editor.home = home

	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}

	if line != "ls ~/Documents/" {
		t.Fatalf("line = %q, want %q", line, "ls ~/Documents/")
	}
}

// And with no home the key does not silently insert something wrong.
func TestLineEditor_tildeWithNoHomeLeavesTheLineAlone(t *testing.T) {
	screen := newScreenModel(t, 70)
	editor := newLineEditor(strings.NewReader("ls ~/Docu\t\r"), screen, t.TempDir())
	editor.width = func() int { return 70 }

	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}

	if line != "ls ~/Docu" {
		t.Fatalf("line = %q, want it unchanged", line)
	}
}
