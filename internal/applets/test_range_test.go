package applets_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// test reads an integer as busybox's getn reads it: one too big for a long long is strtoll's
// ERANGE, `out of range`, which getn says before it looks for a bad number. It was a bad
// number. Either is status 2. Measured against busybox-w32.
func TestTest_saysANumberTooBigIsOutOfRange(t *testing.T) {
	for operand, want := range map[string]string{
		"99999999999999999999":  "99999999999999999999: out of range",
		"-99999999999999999999": "-99999999999999999999: out of range",
		"12x":                   "12x: bad number",
	} {
		_, _, err := runSmall(t, t.TempDir(), "", "test", operand, "-gt", "1")
		status, _ := applets.StatusCode(err)
		if err == nil || err.Error() != want || status != 2 {
			t.Errorf("test %s -gt 1 = %v, status %d; want %q, status 2", operand, err, status, want)
		}
	}
}
