package applets_test

import "testing"

// CONVFMT and OFMT take what busybox's fmt_num takes: a float conversion as printf does it, and
// an integer one of the number's integer part, `CONVFMT = "%d"` keying a[3.7] as 3. They went to
// Go's fmt, which wrote `%!d(float64=3.7)`. Each answer is busybox-w32's, measured, and gawk's,
// which warns of a bad specification and answers the same.
func TestAwk_convfmtAndOfmtAreCFormats(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		program, want string
	}{
		{program: `BEGIN { CONVFMT = "%d"; a[3.7] = 1; for (k in a) print k }`, want: "3\n"},
		{program: `BEGIN { OFMT = "%x"; print 255.5 }`, want: "ff\n"},
		{program: `BEGIN { OFMT = "%i"; print -2.5 }`, want: "-2\n"},
		{program: `BEGIN { CONVFMT = "%5.1f"; x = 3.14159 ""; print "[" x "]" }`, want: "[  3.1]\n"},
		{program: `BEGIN { OFMT = "%.2f"; print 3.14159 }`, want: "3.14\n"},
		{program: `BEGIN { print 3.14159265 }`, want: "3.14159\n"},
	} {
		if got, _, err := runSmall(t, dir, "", "awk", test.program); got != test.want || err != nil {
			t.Errorf("awk %q = %q, %v; want %q", test.program, got, err, test.want)
		}
	}
}
