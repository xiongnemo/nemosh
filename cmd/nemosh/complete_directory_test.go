package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Completion has to be given a directory the host can read, and the shell's own
// answer is not one.
//
// Runtime.WorkingDirectory speaks the shell's path vocabulary -- on Windows
// `/c/Users/nemo/...` -- which is correct for a prompt and unopenable by
// os.ReadDir. Handing it straight to the completer made every file completion in
// every real session return nothing, from the first prompt on. Nothing caught it
// because each existing test built the editor with a native path from t.TempDir,
// so the two vocabularies never met in a test.
//
// This one deliberately starts from the runtime rather than from a path, which
// is the only arrangement that can see the seam.
func TestCompletionDirectory_isReadableByTheHost(t *testing.T) {
	// Given: a real directory with known contents, reached the way a user
	// reaches it -- by running `cd`.
	directory := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	if status := rt.RunScript(context.Background(), "cd "+filepath.ToSlash(directory)+"\n"); status != 0 {
		t.Fatalf("cd into the fixture exited %d", status)
	}

	// When
	where := completionDirectory(rt)

	// Then: the host can read it, and it is the directory `cd` moved to.
	entries, err := os.ReadDir(where)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) = %v -- completion was handed a path the host cannot open", where, err)
	}
	if len(entries) != 3 {
		t.Fatalf("os.ReadDir(%q) found %d entries, want the 3 that were created", where, len(entries))
	}

	// And completion itself answers, which is the behaviour the user sees.
	forCd, _ := completeOperand(completionPaths{workingDirectory: where}, "cd", "")
	if want := []string{"alpha/", "beta/"}; !slices.Equal(forCd, want) {
		t.Fatalf("completeOperand = %v, want %v", forCd, want)
	}
	forCat, _ := completeOperand(completionPaths{workingDirectory: where}, "cat", "")
	if want := []string{"alpha/", "beta/", "notes.txt"}; !slices.Equal(forCat, want) {
		t.Fatalf("completeOperand = %v, want %v", forCat, want)
	}
}

// The shell's own answer must remain the unreadable one, or the test above is
// asserting nothing. If WorkingDirectory ever starts returning a native path,
// this fails and says so rather than quietly making the conversion dead code.
func TestWorkingDirectory_isTheShellsViewNotTheHosts(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	shellView := rt.WorkingDirectory()
	native := completionDirectory(rt)

	// Then
	if shellView == native {
		t.Skip("this platform spells both the same, so there is no seam to cross")
	}
	if _, err := os.ReadDir(shellView); err == nil {
		t.Fatalf("os.ReadDir(%q) succeeded; WorkingDirectory now returns a host path, so completionDirectory is dead code", shellView)
	}
}
