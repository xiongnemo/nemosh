package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func seedPathDirectory(t *testing.T, names ...string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func settledIndex(t *testing.T, pathValue string) *pathIndex {
	t.Helper()
	index := newPathIndex()
	index.refresh(pathValue)
	waitForPathIndex(t, index)
	return index
}

// A program on PATH is a program that runs, so it must be findable by the name a
// person types. `wsl.exe` is typed `wsl`, and answering only to the full name
// would leave the complaint that started this exactly where it was.
func TestPathIndex_findsAProgramByTheNameYouType(t *testing.T) {
	// Given
	index := settledIndex(t, seedPathDirectory(t, "wsl.exe", "git.exe", "notes.txt"))

	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "wsl", want: true},
		{name: "wsl.exe", want: true},
		{name: "git", want: true},
		{name: "nosuchprogram", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			found, ready := index.has(test.name)

			// Then
			if !ready {
				t.Fatal("the index reported itself unready after settling")
			}
			if found != test.want {
				t.Fatalf("has(%q) = %v, want %v", test.name, found, test.want)
			}
		})
	}
}

// An empty PATH is a real value, not a missing one. Treating it as "a build is
// already under way" left the index permanently unready, and every command drawn
// plainly forever.
func TestPathIndex_buildsForAnEmptyPath(t *testing.T) {
	// Given / When
	index := settledIndex(t, "")

	// Then
	found, ready := index.has("anything")
	if !ready {
		t.Fatal("an empty PATH never finished indexing")
	}
	if found {
		t.Fatal("an empty PATH found something")
	}
}

// Until it has read PATH the index says so, rather than saying no. A name drawn
// red and then green a moment later is a colour you learn to ignore.
func TestPathIndex_saysUndeterminedBeforeItHasRead(t *testing.T) {
	// Given: never refreshed
	index := newPathIndex()

	// When
	_, ready := index.has("wsl")

	// Then
	if ready {
		t.Fatal("an index that has not read PATH claimed to know")
	}
	commands := newShellCommands(index)
	if got := commands.standing("nosuchprogram"); got != standingUndetermined {
		t.Fatalf("standing = %v, want undetermined while PATH is unread", got)
	}
	// What the shell carries itself is answerable without PATH, so it is.
	if got := commands.standing("echo"); got != standingRunnable {
		t.Fatalf("standing(echo) = %v, want runnable regardless of PATH", got)
	}
}

// A name that changed PATH gets a fresh index; a name that did not does not pay
// for one.
func TestPathIndex_rebuildsOnlyWhenPathChanges(t *testing.T) {
	// Given
	first := seedPathDirectory(t, "alpha.exe")
	second := seedPathDirectory(t, "beta.exe")
	index := settledIndex(t, first)
	if found, _ := index.has("alpha"); !found {
		t.Fatal("the first PATH was not indexed")
	}

	// When: the same value again, then a different one
	index.refresh(first)
	if found, _ := index.has("alpha"); !found {
		t.Fatal("refreshing with an unchanged PATH discarded the index")
	}
	index.refresh(second)
	deadline := time.Now().Add(10 * time.Second)
	for index.builtFrom() != second {
		if time.Now().After(deadline) {
			t.Fatal("a changed PATH never finished re-indexing")
		}
		time.Sleep(time.Millisecond)
	}

	// Then
	if found, _ := index.has("beta"); !found {
		t.Fatal("a changed PATH was not re-indexed")
	}
	if found, _ := index.has("alpha"); found {
		t.Fatal("the old PATH's programs survived the rebuild")
	}
}

// The session's own aliases and functions are runnable too, and were drawn as
// errors while working perfectly.
func TestShellCommands_countAliasesAndFunctions(t *testing.T) {
	// Given
	commands := newShellCommands(settledIndex(t, ""))
	commands.set([]string{"ll", "gs"})

	// Then
	for _, name := range []string{"ll", "gs", "echo", "cd"} {
		if got := commands.standing(name); got != standingRunnable {
			t.Fatalf("standing(%q) = %v, want runnable", name, got)
		}
	}
	if got := commands.standing("nosuchthing"); got != standingUnknown {
		t.Fatalf("standing of an unknown name = %v, want unknown", got)
	}
}

// A word with a separator in it names a file, not a command, and whether it runs
// is a question about that file rather than about PATH.
func TestShellCommands_saysNothingAboutAPath(t *testing.T) {
	commands := newShellCommands(settledIndex(t, ""))
	for _, name := range []string{"./script.sh", "sub/tool", `sub\tool`} {
		if got := commands.standing(name); got != standingUndetermined {
			t.Fatalf("standing(%q) = %v, want undetermined", name, got)
		}
	}
}

// And PATH feeds the suggestion engine, which is the other half of the request.
func TestShellCommands_offerPathProgramsAsCandidates(t *testing.T) {
	// Given
	commands := newShellCommands(settledIndex(t, seedPathDirectory(t, "wsl.exe")))
	commands.set([]string{"ll"})

	// When
	candidates := commands.candidates()

	// Then
	for _, want := range []string{"ll", "echo", "wsl"} {
		if !slices.Contains(candidates, want) {
			t.Fatalf("candidates do not include %q", want)
		}
	}

	// And a suggestion actually comes out of it.
	if got := (suggester{commands: candidates}).suggest("ws"); got != "l" {
		t.Fatalf("suggest(ws) = %q, want the rest of wsl", got)
	}
}
