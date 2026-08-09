package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The first word of a line names a command; anything after it names a file.
// Completing the wrong one is worse than completing nothing, because the user
// has to notice and undo it.
func TestCompletionContext(t *testing.T) {
	for _, test := range []struct {
		name    string
		line    string
		command bool
	}{
		{name: "an empty line", line: "", command: true},
		{name: "the first word", line: "ec", command: true},
		{name: "after a blank", line: "cat fi"},
		{name: "the second word from empty", line: "cat "},
		{name: "after a pipe a command starts again", line: "cat x | gr", command: true},
		{name: "after a semicolon too", line: "cd /tmp; l", command: true},
		{name: "after && as well", line: "true && ec", command: true},
		{name: "a redirect target is a file", line: "echo hi > ou"},
		{name: "leading blanks do not make it an argument", line: "   ec", command: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := completesCommand(test.line); got != test.command {
				t.Fatalf("completesCommand(%q) = %v, want %v", test.line, got, test.command)
			}
		})
	}
}

// Command completion offers builtins and applets, which is what this shell can
// actually run without consulting PATH.
// shellOwnCommands is the candidate set these tests are about: what the shell
// carries itself, without PATH, which varies by machine.
func shellOwnCommands() []string { return commandNames() }

func TestCompleteCommand_offersBuiltinsAndApplets(t *testing.T) {
	// When
	matches := completeCommand("ec", shellOwnCommands())

	// Then
	if !slices.Contains(matches, "echo") {
		t.Fatalf("completeCommand(%q) = %v, want it to offer echo", "ec", matches)
	}
	for _, match := range matches {
		if !strings.HasPrefix(match, "ec") {
			t.Fatalf("completeCommand(%q) offered %q, which does not match", "ec", match)
		}
	}
	if !slices.IsSorted(matches) {
		t.Fatalf("completeCommand(%q) = %v, want it sorted", "ec", matches)
	}
}

func TestCompleteCommand_offersBuiltins(t *testing.T) {
	// When
	matches := completeCommand("expo", shellOwnCommands())

	// Then
	if !slices.Contains(matches, "export") {
		t.Fatalf("completeCommand(%q) = %v, want it to offer the export builtin", "expo", matches)
	}
}

func TestCompleteCommand_offersNothingForAnUnknownPrefix(t *testing.T) {
	if matches := completeCommand("zzzznosuch", shellOwnCommands()); len(matches) != 0 {
		t.Fatalf("completeCommand offered %v, want nothing", matches)
	}
}

// File completion is relative to where the shell actually is, and a directory
// is offered with a trailing slash so the next Tab can descend into it.
func TestCompleteFile(t *testing.T) {
	// Given
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "alpine.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "alphadir"), 0o700); err != nil {
		t.Fatal(err)
	}

	// When
	matches := completeFile(directory, "alp")

	// Then
	want := []string{"alpha.txt", "alphadir/", "alpine.txt"}
	if !slices.Equal(matches, want) {
		t.Fatalf("completeFile = %v, want %v", matches, want)
	}
}

func TestCompleteFile_descendsIntoADirectoryPrefix(t *testing.T) {
	// Given
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "sub", "inner.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	matches := completeFile(directory, "sub/in")

	// Then
	if !slices.Equal(matches, []string{"sub/inner.txt"}) {
		t.Fatalf("completeFile = %v, want %v", matches, []string{"sub/inner.txt"})
	}
}

func TestCompleteFile_offersEverythingForAnEmptyPrefix(t *testing.T) {
	// Given
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "only.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// When / Then
	if matches := completeFile(directory, ""); !slices.Equal(matches, []string{"only.txt"}) {
		t.Fatalf("completeFile = %v, want %v", matches, []string{"only.txt"})
	}
}

// The shared prefix is what Tab inserts when several candidates match: it takes
// the user as far as the choice actually is, without picking for them.
func TestLongestSharedPrefix(t *testing.T) {
	for _, test := range []struct {
		name    string
		matches []string
		want    string
	}{
		{name: "none", matches: nil, want: ""},
		{name: "one is itself", matches: []string{"echo"}, want: "echo"},
		{name: "a common stem", matches: []string{"alpha.txt", "alpine.txt"}, want: "alp"},
		{name: "nothing in common", matches: []string{"abc", "xyz"}, want: ""},
		{name: "one is a prefix of the other", matches: []string{"ls", "lsattr"}, want: "ls"},
		{name: "multi-byte runes are not split", matches: []string{"你好世界", "你好朋友"}, want: "你好"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := longestSharedPrefix(test.matches); got != test.want {
				t.Fatalf("longestSharedPrefix(%v) = %q, want %q", test.matches, got, test.want)
			}
		})
	}
}
