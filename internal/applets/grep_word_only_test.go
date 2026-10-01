package applets_test

import (
	"testing"
)

// -w -o prints each word alone, a word one space after another included, and none that a word
// character touches. It printed the characters around a word with it, `apple ` and ` foo`, and
// missed the second of two words one space apart. busybox-w32's grep printed the words, and a
// fragment of a word it passed over, `ar `, which is not followed; GNU's answers are these.
func TestGrep_wordOnlyPrintsTheWordsAlone(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"w.txt": "apple apple\nfoo_bar foo\nxfoo foo.\n"})
	stdout, _, err := runSmall(t, dir, "", "grep", "-ow", `apple\|foo`, "w.txt")
	if want := "apple\napple\nfoo\nfoo\n"; stdout != want || err != nil {
		t.Errorf("grep -ow = %q, %v; want %q", stdout, err, want)
	}
	stdout, _, err = runSmall(t, dir, "", "grep", "-cw", "foo", "w.txt")
	if stdout != "2\n" || err != nil {
		t.Errorf("grep -cw foo = %q, %v; want the two lines a whole foo is on", stdout, err)
	}
}
