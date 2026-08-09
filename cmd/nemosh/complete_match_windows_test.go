//go:build windows

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// NTFS does not distinguish case, so a shell that does while completing is
// contradicting the directory it is reading: `Program Files` is openable as
// `program files`, and refusing to offer it for `prog` is an answer the
// filesystem disagrees with. busybox-w32 makes the same split
// (libbb/lineedit.c:1039), and this shell already carries `set -o nocaseglob`
// for the same reason.
func TestCompletion_ignoresCase_becauseTheFilesystemDoes(t *testing.T) {
	// Given
	directory := t.TempDir()
	for _, name := range []string{"Program Files", "Windows"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name  string
		typed string
		want  []string
	}{
		{name: "lower for an upper name", typed: "prog", want: []string{"Program Files/"}},
		{name: "upper for an upper name", typed: "Prog", want: []string{"Program Files/"}},
		{name: "lower for a mixed name", typed: "readme", want: []string{"README.md"}},
		{name: "no match is still no match", typed: "zzz", want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := completeFile(directory, test.typed); !slices.Equal(got, test.want) {
				t.Fatalf("completeFile(%q) = %v, want %v", test.typed, got, test.want)
			}
		})
	}

	// And the name is offered as it is spelled on disk, not as it was typed --
	// inserting `program files/` would work but would leave the line lying
	// about what is there.
	if got := completeFile(directory, "prog"); len(got) != 1 || got[0] != "Program Files/" {
		t.Fatalf("completeFile(\"prog\") = %v, want the on-disk spelling", got)
	}
}
