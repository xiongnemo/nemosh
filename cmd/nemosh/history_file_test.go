package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

// Where history is kept, and when it is not kept at all.
func TestNewHistoryFile(t *testing.T) {
	home := t.TempDir()
	tests := []struct {
		name    string
		env     map[string]string
		path    string
		limit   int
		enabled bool
	}{
		{
			name: "the default beside the home directory",
			env:  map[string]string{"HOME": home},
			path: filepath.Join(home, ".nemosh_history"), limit: 500, enabled: true,
		},
		{
			name: "somewhere else entirely",
			env:  map[string]string{"HOME": home, "HISTFILE": "/tmp/elsewhere"},
			path: "/tmp/elsewhere", limit: 500, enabled: true,
		},
		{
			// Set and empty means off. bash's rule, and busybox spells the same
			// special case: an explicit empty value is a decision.
			name: "set to nothing, which turns it off",
			env:  map[string]string{"HOME": home, "HISTFILE": ""},
			path: "", limit: 500, enabled: false,
		},
		{
			name: "a limit of zero also turns it off",
			env:  map[string]string{"HOME": home, "HISTFILESIZE": "0"},
			path: filepath.Join(home, ".nemosh_history"), limit: 0, enabled: false,
		},
		{
			name: "a limit that is not a number is ignored",
			env:  map[string]string{"HOME": home, "HISTFILESIZE": "lots"},
			path: filepath.Join(home, ".nemosh_history"), limit: 500, enabled: true,
		},
		{
			// Nowhere to put it. Not an error: a shell with no HOME still works,
			// it just forgets.
			name: "no home to keep it in",
			env:  map[string]string{},
			path: "", limit: 500, enabled: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			file := newHistoryFile(lookupFrom(test.env), nil)

			// Then
			if file.path != test.path || file.limit != test.limit || file.enabled() != test.enabled {
				t.Fatalf("historyFile = %+v (enabled %v), want path %q limit %d enabled %v",
					file, file.enabled(), test.path, test.limit, test.enabled)
			}
		})
	}
}

// A line at a time, appended, so a session that is killed still leaves what it
// ran and two windows do not overwrite each other.
func TestHistoryFile_appendsAndReloads(t *testing.T) {
	// Given
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil)

	// When
	file.append("ssh gpu-worker-34")
	file.append("ls -al")
	file.append("   ")

	// Then
	if got := file.load(); !slices.Equal(got, []string{"ssh gpu-worker-34", "ls -al"}) {
		t.Fatalf("load = %q, want the two real lines", got)
	}
	// A second shell sees it, which is the whole point.
	other := newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil)
	if got := other.load(); len(got) != 2 {
		t.Fatalf("a second session loaded %q", got)
	}
}

// A command spanning lines is stored as typed, so writing it raw would make one
// entry read back as several. Recalling half a loop is worse than not recalling
// it.
func TestHistoryFile_refusesAMultiLineCommand(t *testing.T) {
	// Given
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil)

	// When
	file.append("for i in a b\ndo echo $i\ndone")
	file.append("echo after")

	// Then
	if got := file.load(); !slices.Equal(got, []string{"echo after"}) {
		t.Fatalf("load = %q, want only the single-line command", got)
	}
}

// Loading returns at most the limit, and the *most recent* of them.
func TestHistoryFile_loadsOnlyTheMostRecent(t *testing.T) {
	// Given
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home, "HISTFILESIZE": "3"}), nil)
	for _, line := range []string{"one", "two", "three", "four", "five"} {
		file.append(line)
	}

	// When
	got := file.load()

	// Then
	if !slices.Equal(got, []string{"three", "four", "five"}) {
		t.Fatalf("load = %q, want the last three", got)
	}
}

// Trimming is lazy: the file is rewritten once it reaches several times the
// limit, not on every command. busybox's rule, and its reason -- rewriting a
// file per command to keep it at exactly N lines is work nobody asked for.
func TestHistoryFile_trimsOnlyOnceTheFileIsFarPastTheLimit(t *testing.T) {
	// Given
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home, "HISTFILESIZE": "2"}), nil)

	// When: five lines, against a limit of 2 and a trim threshold of 2*4
	for _, line := range []string{"a", "b", "c", "d", "e"} {
		file.append(line)
	}

	// Then: still all five on disk, because 5 is not past 8
	if got := file.readAll(); len(got) != 5 {
		t.Fatalf("file holds %q, want all five still there", got)
	}

	// When: past the threshold
	for _, line := range []string{"f", "g", "h", "i"} {
		file.append(line)
	}

	// Then: trimmed to the limit, keeping the newest
	if got := file.readAll(); !slices.Equal(got, []string{"h", "i"}) {
		t.Fatalf("file holds %q, want the last two", got)
	}
}

// A file that cannot be read is a first run, not a failure. busybox says the
// same thing in a comment: do not trash old history if the file cannot be
// opened.
func TestHistoryFile_isSilentAboutAMissingFile(t *testing.T) {
	// Given
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": filepath.Join(t.TempDir(), "nothing-here")}), nil)

	// When
	got := file.load()

	// Then
	if got != nil {
		t.Fatalf("load = %q, want nothing", got)
	}
}

// Nothing is written when saving is off.
func TestHistoryFile_writesNothingWhenDisabled(t *testing.T) {
	// Given
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home, "HISTFILE": ""}), nil)

	// When
	file.append("secret")

	// Then
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "history") {
			t.Fatalf("wrote %s with saving turned off", entry.Name())
		}
	}
}
