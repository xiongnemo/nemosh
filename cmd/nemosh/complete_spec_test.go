package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func seedCompletionTree(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"notes.txt", "alpine.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

// `cd notes.txt` cannot work, so offering it is worse than offering nothing:
// the user has to read the list, reject most of it, and the shared prefix that
// Tab inserts is dragged toward candidates that were never possible.
func TestCompleteOperand_offersOnlyDirectories_forCommandsThatTakeThem(t *testing.T) {
	directory := seedCompletionTree(t)
	for _, test := range []struct {
		command string
		want    []string
	}{
		{command: "cd", want: []string{"alpha/", "beta/"}},
		{command: "rmdir", want: []string{"alpha/", "beta/"}},
		{command: "mkdir", want: []string{"alpha/", "beta/"}},
		{command: "ls", want: []string{"alpha/", "alpine.txt", "beta/", "notes.txt"}},
		{command: "cat", want: []string{"alpha/", "alpine.txt", "beta/", "notes.txt"}},
		{command: "", want: []string{"alpha/", "alpine.txt", "beta/", "notes.txt"}},
	} {
		t.Run(test.command, func(t *testing.T) {
			if got, _ := completeOperand(completionPaths{workingDirectory: directory}, test.command, ""); !slices.Equal(got, test.want) {
				t.Fatalf("completeOperand(%q) = %v, want %v", test.command, got, test.want)
			}
		})
	}
}

// A directory-only command still narrows by what has been typed, and the shared
// prefix it inserts must come from the directories alone. With files included,
// `cd al` would insert the prefix shared by `alpha/` and `alpine.txt` -- `alp` --
// and stop, where the only possible answer was `alpha/`.
func TestCompleteOperand_sharedPrefixIgnoresImpossibleCandidates(t *testing.T) {
	// Given
	directory := seedCompletionTree(t)

	// When
	forCd, _ := completeOperand(completionPaths{workingDirectory: directory}, "cd", "al")
	forLs, _ := completeOperand(completionPaths{workingDirectory: directory}, "ls", "al")

	// Then
	if !slices.Equal(forCd, []string{"alpha/"}) {
		t.Fatalf("cd offered %v, want just alpha/", forCd)
	}
	if got := longestSharedPrefix(forCd); got != "alpha/" {
		t.Fatalf("cd would insert %q, want the whole of alpha/", got)
	}
	if got := longestSharedPrefix(forLs); got != "alp" {
		t.Fatalf("ls would insert %q, want alp -- the choice is real there", got)
	}
}

func TestCommandInProgress(t *testing.T) {
	for _, test := range []struct {
		prefix string
		want   string
	}{
		{prefix: "", want: ""},
		{prefix: "cd ", want: "cd"},
		{prefix: "cd  ", want: "cd"},
		{prefix: "ls -l ", want: "ls"},
		{prefix: "ls | cd ", want: "cd"},
		{prefix: "echo a; cd ", want: "cd"},
		{prefix: "true && cd ", want: "cd"},
		{prefix: "(cd ", want: "cd"},
		{prefix: "cat notes.txt | grep ", want: "grep"},
	} {
		t.Run(test.prefix, func(t *testing.T) {
			if got := commandInProgress(test.prefix); got != test.want {
				t.Fatalf("commandInProgress(%q) = %q, want %q", test.prefix, got, test.want)
			}
		})
	}
}

// A completed name has to be usable, not merely correct. On Windows a blank in
// a path is the common case, not the exotic one, and inserting `Program Files/`
// raw produces a command naming two operands, neither of which exists.
func TestCompletion_escapesWhatItInserts_andMatchesWhatWasTyped(t *testing.T) {
	// Given
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "My Documents"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When
	screen := newScreenModel(t, 60)
	editor := newLineEditor(strings.NewReader("cd My\t\r"), screen, directory)
	editor.width = func() int { return 60 }
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}

	// Then: one operand, not two.
	if want := `cd My\ Documents/`; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}

// The second Tab has to match against the name on disk rather than the escape
// on screen, or completing a path with a blank in it can only ever be done once.
func TestCompletion_continuesFromAnEscapedWord(t *testing.T) {
	// Given
	directory := t.TempDir()
	nested := filepath.Join(directory, "My Documents")
	if err := os.MkdirAll(filepath.Join(nested, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When: the blank arrives already escaped, as the first completion left it
	screen := newScreenModel(t, 60)
	editor := newLineEditor(strings.NewReader(`cd My\ Documents/re`+"\t\r"), screen, directory)
	editor.width = func() int { return 60 }
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}

	// Then
	if want := `cd My\ Documents/reports/`; line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}

func TestEscapeForInsertion(t *testing.T) {
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{name: "nothing special", in: "notes.txt", want: "notes.txt"},
		{name: "a blank", in: "My Documents/", want: `My\ Documents/`},
		{name: "a dollar", in: "$HOME.bak", want: `\$HOME.bak`},
		{name: "a quote", in: "it's", want: `it\'s`},
		// `[` opens a bracket expression and has to be escaped; a lone `]` is
		// ordinary text, and busybox does not escape it either.
		{name: "a glob", in: "a[1].txt", want: `a\[1].txt`},
		{name: "wide runes are untouched", in: "文档/", want: "文档/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := escapeForInsertion(test.in)
			if got != test.want {
				t.Fatalf("escapeForInsertion(%q) = %q, want %q", test.in, got, test.want)
			}
			if round := unescapeTypedWord(got); round != test.in {
				t.Fatalf("unescapeTypedWord(%q) = %q, want %q", got, round, test.in)
			}
		})
	}
}
