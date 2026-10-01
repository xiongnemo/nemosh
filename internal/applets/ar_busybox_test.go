package applets_test

import "testing"

// ar takes c and does nothing with it, as busybox's getopt string "voc" takes it, so `ar rc`,
// GNU's way of writing it, makes the archive; c was an invalid option. An archive ar cannot
// open is `cannot open 'X'`, in the shape of busybox's xopen; it was `X: ...`. Each answer was
// measured against busybox-w32.
func TestAr_takesCAndSaysWhatItCannotOpen(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "hello\n"})
	if _, stderr, err := runSmall(t, dir, "", "ar", "rc", "t.a", "a.txt"); err != nil {
		t.Fatalf("ar rc: %v (%s)", err, stderr)
	}
	if got, stderr, err := runSmall(t, dir, "", "ar", "tc", "t.a"); got != "a.txt\n" || err != nil {
		t.Errorf("ar tc t.a = %q, %v (%s); want a.txt", got, err, stderr)
	}
	want := "cannot open 'nope.a': No such file or directory"
	if _, _, err := runSmall(t, dir, "", "ar", "t", "nope.a"); err == nil || err.Error() != want {
		t.Errorf("ar t nope.a = %v; want %q", err, want)
	}
}
