package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// tac needs expectations of its own rather than a place in the preserve/normalise
// table, because it *reorders*: "the output equals the input" is not its property
// and no category can express what is.
//
// Every expectation here is measured from busybox, which nemosh already matched --
// this is a test of something that was right, written because the property table
// tried to classify tac and could not, and a wrong classification is worse than an
// explicit list.
//
// The interesting one is the first. An unterminated final line becomes the *first*
// output line, and the terminator ends up at the end: `a\nb` answers `ba\n`. So the
// endings stay where the endings were rather than travelling with their lines --
// which reads as odd and is what both this build and busybox do.
func TestTac_matchesBusyboxOnEveryEndingShape(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{name: "no final newline", input: "a\nb", want: "ba\n"},
		{name: "final newline", input: "a\nb\n", want: "b\na\n"},
		{name: "CRLF", input: "a\r\nb\r\n", want: "b\r\na\r\n"},
		{name: "one line", input: "only\n", want: "only\n"},
		{name: "one unterminated line", input: "only", want: "only"},
		{name: "empty", input: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "in.txt"), []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := runSmall(t, root, "", "tac", "in.txt")
			if err != nil {
				t.Fatalf("tac: %v (%s)", err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("tac on %q = %q, want %q", test.input, stdout, test.want)
			}
		})
	}
}

// sed -i turns a CRLF file into LF, and both references do the same: GNU and
// busybox each answer four bytes for a six-byte CRLF file. So this is not a defect
// to fix but a behaviour to know about, and on a Windows-first machine it is worth
// knowing -- `sed -i 's/x/y/' notes.txt` rewrites every line ending in the file
// even when nothing matched.
//
// Pinned here so that a future change to sed's reading has to decide about it on
// purpose rather than by accident.
func TestSed_inPlaceNormalisesCrlfLikeBothReferences(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "in.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, root, "", "sed", "-i", "s/zzzznosuch/x/", "in.txt"); err != nil {
		t.Fatalf("sed -i: %v (%s)", err, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != "a\nb\n" {
		t.Fatalf("sed -i on a CRLF file = %q, want %q, which is what GNU and busybox both answer",
			after, "a\nb\n")
	}
}
