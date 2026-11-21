package applets_test

import (
	"testing"
)

// What comes before a conversion printf cannot do is written before it fails, as busybox and
// bash write it; it wrote nothing. Measured against busybox-w32. `%k` is no conversion in
// either; `%z`, which this was written with, is C's length modifier, which both pass over.
func TestPrintf_writesWhatCameBeforeABadConversion(t *testing.T) {
	stdout, _, err := runSmall(t, t.TempDir(), "", "printf", "a%kb\n")
	if stdout != "a" || err == nil || err.Error() != "invalid conversion specification %k" {
		t.Errorf("printf 'a%%kb' = %q, %v; want a, and the conversion refused", stdout, err)
	}
	stdout, _, err = runSmall(t, t.TempDir(), "", "printf", "%s|%k|", "x")
	if stdout != "x|" || err == nil {
		t.Errorf("printf '%%s|%%k|' x = %q, %v; want x|, and the conversion refused", stdout, err)
	}
}
