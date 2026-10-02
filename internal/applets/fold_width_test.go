package applets_test

import "testing"

// fold reads -w as busybox's xatou_range(w_opt, 1, 10000) reads it: a width outside the range
// is `number N is not in 1..10000 range`, and one that is no number `invalid number`. Each was
// "illegal width value", and 10001 was taken. Measured against busybox-w32.
func TestFold_readsTheWidthAsBusyboxDoes(t *testing.T) {
	for width, want := range map[string]string{
		"0":     "number 0 is not in 1..10000 range",
		"10001": "number 10001 is not in 1..10000 range",
		"abc":   "invalid number 'abc'",
		"-3":    "invalid number '-3'",
		"5x":    "invalid number '5x'",
	} {
		if _, _, err := runSmall(t, t.TempDir(), "a\n", "fold", "-w", width); err == nil || err.Error() != want {
			t.Errorf("fold -w %s = %v; want %q", width, err, want)
		}
	}
	if out, _, err := runSmall(t, t.TempDir(), "abcdef\n", "fold", "-w", "10000"); out != "abcdef\n" || err != nil {
		t.Errorf("fold -w 10000 = %q, %v", out, err)
	}
}
