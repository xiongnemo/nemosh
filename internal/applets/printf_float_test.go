package applets_test

import "testing"

// %g has six significant digits unless the precision says, as C's has; Go's has as many as
// read back, and `printf %g 123456789` was 1.23456789e+08. inf is spelled inf, INF under %F
// and %G, as C spells it; it was Go's +Inf. Each answer is bash's, and busybox-w32's but for
// its three-digit exponents and 1.#INF, which are the MSVC runtime's.
func TestPrintf_floatsAreCs(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{`%g|%g|%g|%g\n`, "123456789", "3.14159265", "0.000012345678", "100"}, want: "1.23457e+08|3.14159|1.23457e-05|100\n"},
		{args: []string{`%G|%G\n`, "123456789", "1e-10"}, want: "1.23457E+08|1E-10\n"},
		{args: []string{`%10g|%-10g|%010g\n`, "3.14159265", "2.5", "1.5"}, want: "   3.14159|2.5       |00000001.5\n"},
		{args: []string{`%g|%.3g|%#g\n`, "999999.5", "3.14159", "1"}, want: "1e+06|3.14|1.00000\n"},
		{args: []string{`%e|%f\n`, "123456789", "123456789.123456789"}, want: "1.234568e+08|123456789.123457\n"},
		{args: []string{`%f|%e|%g|%F|%G\n`, "inf", "inf", "inf", "inf", "-inf"}, want: "inf|inf|inf|INF|-INF\n"},
		{args: []string{`%5.1f|%-6g|\n`, "inf", "inf"}, want: "  inf|inf   |\n"},
	} {
		if got, _, err := runSmall(t, dir, "", "printf", test.args...); got != test.want || err != nil {
			t.Errorf("printf %q = %q, %v; want %q", test.args, got, err, test.want)
		}
	}
}

// awk's printf has C's %g as printf has, and spells infinity as its print does, gawk's +inf,
// upper case under %E %F %G. Each answer is gawk 5.4's; busybox-w32's %g is the same but for
// its MSVC exponents.
func TestAwk_printfFloatsAreCs(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		program, want string
	}{
		{program: `BEGIN { printf "%g|%G|%.3g|%10.4g|\n", 123456789, 0.000012345678, 3.14159, 2.5 }`, want: "1.23457e+08|1.23457E-05|3.14|       2.5|\n"},
		{program: `BEGIN { x = -log(0); printf "%E|%G|%F|%e|%5.1f|%-6g|%+g\n", x, -x, x, x, x, x, x }`, want: "+INF|-INF|+INF|+inf| +inf|+inf  |+inf\n"},
	} {
		if got, _, err := runSmall(t, dir, "", "awk", test.program); got != test.want || err != nil {
			t.Errorf("awk %q = %q, %v; want %q", test.program, got, err, test.want)
		}
	}
}
