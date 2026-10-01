package applets_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// -w passes over every white space character, and -b reads a run of them as one, white space at
// a line's end being no difference and a run at its start one; -B passes over a hunk that adds
// or removes empty lines only, a line of a space being no empty one. Each answer is
// busybox-w32's, measured: -w read a run as one space, which is -b's meaning, and -b and -B were
// taken and ignored.
func TestDiff_comparesWhiteSpaceAndEmptyLinesAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		options     []string
		left, right string
		same        bool
	}{
		{options: []string{"-w"}, left: "ab\n", right: "a b\n", same: true},
		{options: []string{"-w"}, left: " a\n", right: "a\n", same: true},
		{options: []string{"-b"}, left: "ab\n", right: "a b\n"},
		{options: []string{"-b"}, left: "a  b\n", right: "a b\n", same: true},
		{options: []string{"-b"}, left: "a\tb\n", right: "a b\n", same: true},
		{options: []string{"-b"}, left: "a \n", right: "a\n", same: true},
		{options: []string{"-b"}, left: " a\n", right: "a\n"},
		{options: []string{"-b"}, left: "  a\n", right: " a\n", same: true},
		{options: []string{"-B"}, left: "a\n\nb\n", right: "a\nb\n", same: true},
		{options: []string{"-B"}, left: "\na\n", right: "a\n", same: true},
		{options: []string{"-B"}, left: "a\n\n\nb\n", right: "a\nb\n", same: true},
		{options: []string{"-B"}, left: "a\n \nb\n", right: "a\nb\n"},
		{options: []string{"-B"}, left: "a\n\nb\n", right: "a\nc\n"},
	} {
		for name, text := range map[string]string{"l": test.left, "r": test.right} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		_, _, err := runSmall(t, dir, "", "diff", append(test.options, "l", "r")...)
		if same := err == nil; same != test.same || !same && !errors.Is(err, applets.ErrExitFalse) {
			t.Errorf("diff %q on %q and %q = %v; the same should be %v", test.options, test.left, test.right, err, test.same)
		}
	}
}

// A hunk -B passes over is not counted, so -s says the files are the same, and a hunk with an
// empty line and a real change is printed whole.
func TestDiff_minusBPrintsWhatItKeepsWhole(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"l": "a\n\nb\n", "r": "a\nb\n", "m": "a\nc\n"})
	if stdout, _, err := runSmall(t, dir, "", "diff", "-s", "-B", "l", "r"); stdout != "Files l and r are identical\n" || err != nil {
		t.Errorf("diff -s -B = %q, %v; want the files identical", stdout, err)
	}
	stdout, _, _ := runSmall(t, dir, "", "diff", "-B", "l", "m")
	if want := "--- l\n+++ m\n@@ -1,3 +1,2 @@\n a\n-\n-b\n+c\n"; stdout != want {
		t.Errorf("diff -B l m = %q; want %q", stdout, want)
	}
}
