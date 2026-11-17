package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// A history file bash wrote under HISTTIMEFORMAT -- ~/.bash_history, named as HISTFILE -- is
// read as bash reads it: each # and digits is the time of the entry after it, not a command.
// They were loaded as commands, and the arrows walked through a column of timestamps.
func TestHistoryFile_readsBashsTimestamps(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".nemosh_history"), []byte("#1700000000\nls -al\n#1700000060\ngit status\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil)

	records, held := file.loadCounted()

	if lines := recordLines(records); !slices.Equal(lines, []string{"ls -al", "git status"}) || held != 2 {
		t.Fatalf("loaded %q, %d entries; want the two commands", lines, held)
	}
}

// Under HISTTIMEFORMAT the session writes each command with its time, as bash does, and then
// a command of several lines is written whole: the times say where it ends. Without it such a
// command is still kept out of the file, since it would read back as several.
func TestHistoryFile_writesAMultiLineCommandUnderHisttimeformat(t *testing.T) {
	home := t.TempDir()
	stamped := newHistoryFile(lookupFrom(map[string]string{"HOME": home, "HISTTIMEFORMAT": ""}), nil)
	loop := "for i in 1\ndo echo $i\ndone"

	if !stamped.writes(loop) {
		t.Fatal("a multi-line command is not written under HISTTIMEFORMAT")
	}
	stamped.append("echo hi")
	stamped.append(loop)

	data, err := os.ReadFile(filepath.Join(home, ".local", "state", "nemosh", "nemosh_history"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^#\d+\necho hi\n#\d+\nfor i in 1\ndo echo \$i\ndone\n$`).Match(data) {
		t.Fatalf("file = %q", data)
	}
	if records, _ := stamped.loadCounted(); !slices.Equal(recordLines(records), []string{"echo hi", loop}) {
		t.Fatalf("read back %q", recordLines(records))
	}
	if plain := newHistoryFile(lookupFrom(map[string]string{"HOME": t.TempDir()}), nil); plain.writes(loop) {
		t.Fatal("a multi-line command is written without HISTTIMEFORMAT, where it would read back as three")
	}
}
