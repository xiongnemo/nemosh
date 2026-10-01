package applets_test

import (
	"testing"
)

// What comes before a conversion printf cannot do is written before it fails, as busybox and
// bash write it; it wrote nothing. Measured against busybox-w32.
func TestPrintf_writesWhatCameBeforeABadConversion(t *testing.T) {
	stdout, _, err := runSmall(t, t.TempDir(), "", "printf", "a%zb\n")
	if stdout != "a" || err == nil || err.Error() != "invalid conversion specification %z" {
		t.Errorf("printf 'a%%zb' = %q, %v; want a, and the conversion refused", stdout, err)
	}
	stdout, _, err = runSmall(t, t.TempDir(), "", "printf", "%s|%z|", "x")
	if stdout != "x|" || err == nil {
		t.Errorf("printf '%%s|%%z|' x = %q, %v; want x|, and the conversion refused", stdout, err)
	}
}
