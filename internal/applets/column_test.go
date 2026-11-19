package applets_test

import "testing"

// column is util-linux's (text-utils/column.c): by default the lines are entries filled into
// columns down and then across, -x across and then down, each column as wide as the widest entry
// rounded up to the next tab stop and padded with tabs; -t a table, whitespace splitting it with
// runs merged, -s's characters each a field's end so an empty field stays one, the columns joined
// by two spaces or -o's separator, the last not padded. Empty lines are left out but with -L.
// There was no column.
func TestColumn_asUtilLinuxDoes(t *testing.T) {
	seven := "one\ntwo\nthree\nfour\nfive\nsix\nseven\n"
	for _, test := range []struct {
		args        []string
		input, want string
	}{
		{[]string{"-c", "40"}, seven, "one\tthree\tfive\tseven\ntwo\tfour\tsix\n"},
		{[]string{"-x", "-c", "40"}, seven, "one\ttwo\tthree\tfour\tfive\nsix\tseven\n"},
		{[]string{"-c", "4"}, "ab\n\ncd\n", "ab\ncd\n"},
		{[]string{"-t"}, "a b c\nlonger   x y\n\nz\n", "a       b  c\nlonger  x  y\nz\n"},
		{[]string{"-t", "-s", ","}, "a,,b\nxx,y,z\n", "a      b\nxx  y  z\n"},
		{[]string{"-t", "-o", " | "}, "a b\nc d\n", "a | b\nc | d\n"},
		{[]string{"-t", "-R", "2"}, "a 1\nb 100\n", "a    1\nb  100\n"},
		{[]string{"-t", "-N", "NAME,SIZE"}, "a 1\nbb 22\n", "NAME  SIZE\na     1\nbb    22\n"},
		{[]string{"-t", "-d", "-N", "NAME,SIZE"}, "a 1\n", "a  1\n"},
		{[]string{"-t", "-l", "2"}, "a b c d\nee f\n", "a   b c d\nee  f\n"},
		{[]string{"-t", "-L"}, "a b\n\nc d\n", "a  b\n\nc  d\n"},
		{[]string{"-t", "-s", ":"}, "中文:1\nab:22\n", "中文  1\nab    22\n"},
	} {
		if stdout, stderr, err := runSmall(t, t.TempDir(), test.input, "column", test.args...); err != nil || stdout != test.want {
			t.Errorf("column %q over %q = %q, %v (%s); want %q", test.args, test.input, stdout, err, stderr, test.want)
		}
	}
}
