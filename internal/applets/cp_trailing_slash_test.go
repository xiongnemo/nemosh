package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A DEST written with a slash after it is a directory or it is nothing, as busybox's cp has it.
// `cp a.txt nope/` fails when nope is not there, and makes no file nope, which it did; a file
// named so is "Not a directory". A directory still copies to such a name, a file still copies
// into a directory that is there, and a SOURCE not there is still the first thing said. Each
// answer is busybox-w32's, measured.
func TestCp_destWithASlashIsADirectory(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "hi\n", "src/f": "x\n", "d/keep": ""})
	for _, test := range []struct {
		args []string
		word string
	}{
		{args: []string{"a.txt", "nope/"}, word: "cannot create 'nope/'"},
		{args: []string{"a.txt", "a.txt/"}, word: "cannot stat 'a.txt/': Not a directory"},
		{args: []string{"missing", "x/"}, word: "cannot stat 'missing'"},
	} {
		_, stderr, err := runSmall(t, dir, "", "cp", test.args...)
		if err == nil || !strings.Contains(stderr+err.Error(), test.word) {
			t.Errorf("cp %q = %q, %v; want a failure saying %q", test.args, stderr, err, test.word)
		}
	}
	if _, err := os.Lstat(filepath.Join(dir, "nope")); err == nil {
		t.Error("cp a.txt nope/ made a file nope")
	}
	for _, test := range []struct {
		args []string
		made string
	}{
		{args: []string{"a.txt", "d/"}, made: "d/a.txt"},
		{args: []string{"-r", "src", "newdir/"}, made: "newdir/f"},
	} {
		if _, stderr, err := runSmall(t, dir, "", "cp", test.args...); err != nil {
			t.Errorf("cp %q: %q, %v", test.args, stderr, err)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(test.made))); err != nil {
			t.Errorf("cp %q made no %s: %v", test.args, test.made, err)
		}
	}
}
