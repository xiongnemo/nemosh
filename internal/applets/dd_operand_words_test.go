package applets_test

import "testing"

// dd reads a block size as busybox-w32's xatoul_range_sfx(val, 1, ULONG_MAX/2) reads it, its
// unsigned long being 32 bits: 0 and 3000000000 are `not in 1..2147483647 range`, where 0 was an
// invalid number and 3000000000 was taken. A value that is no number is named whole, `bs=x`
// having said `invalid number` with nothing in its quotes, and an operand dd does not have is
// quoted whole, as GNU's quotes it, `bogus=1`, where busybox's shows its usage. Measured
// against both.
func TestDd_namesABadOperandAsBusyboxDoes(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"bs=0", "if=/dev/null"}, want: "number 0 is not in 1..2147483647 range"},
		{args: []string{"obs=0", "if=/dev/null"}, want: "number 0 is not in 1..2147483647 range"},
		{args: []string{"ibs=3000000000", "if=/dev/null"}, want: "number 3000000000 is not in 1..2147483647 range"},
		{args: []string{"bs=x", "if=/dev/null"}, want: "invalid number 'x'"},
		{args: []string{"count=x", "if=/dev/null"}, want: "invalid number 'x'"},
		{args: []string{"bogus=1"}, want: "unrecognized operand 'bogus=1'"},
	} {
		_, _, err := runSmall(t, t.TempDir(), "", "dd", test.args...)
		if err == nil || err.Error() != test.want {
			t.Errorf("dd %q = %v; want %q", test.args, err, test.want)
		}
	}
	if out, _, err := runSmall(t, t.TempDir(), "abcdefgh", "dd", "bs=2x2", "count=1"); out != "abcd" || err != nil {
		t.Errorf("dd bs=2x2 count=1 = %q, %v; want abcd", out, err)
	}
}
