package applets_test

import "testing"

// join is busybox's merge join: sets of lines sharing a key, read from files sorted on their join
// fields, with -a -v -e -o and -t. It paired every line with every line, which joined unsorted
// files busybox does not, took -t and split on blanks anyway, and refused the rest. Each answer is
// busybox-w32's, measured, but for `-o 0`, where busybox reads past its list and prints the key
// six times.
func TestJoin_isBusyboxsMergeJoin(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"f1": "a 1\nb 2\nc 3\n", "f2": "a A\nc C\nd D\n",
		"t1": "k:1:x\nm::y\n", "t2": "k:2\nm:3\n",
		"m1": "a 1\na 2\n", "m2": "a A\na B\na C\n",
		"u1": "b 1\na 2\n", "u2": "a A\nb B\n",
		"b1": "\na 1\n", "b2": "\na A\n",
		"r1": "a 1\r\nb 2\r\n", "r2": "a A\r\nb B\r\n",
		"s1": "a  1 x\n", "s2": "a 2\n",
		"e1": "k::x\n", "e2": "k:\n",
		"c1": "B 1\na 2\n", "c2": "B x\na y\n",
	})
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"f1", "f2"}, "a 1 A\nc 3 C\n"},
		{[]string{"-a1", "f1", "f2"}, "a 1 A\nb 2\nc 3 C\n"},
		{[]string{"-a", "2", "f1", "f2"}, "a 1 A\nc 3 C\nd D\n"},
		{[]string{"-a1", "-a2", "f1", "f2"}, "a 1 A\nb 2\nc 3 C\nd D\n"},
		{[]string{"-v1", "f1", "f2"}, "b 2\n"},
		{[]string{"-v1", "-v2", "f1", "f2"}, "b 2\nd D\n"},
		{[]string{"f1", "f2", "-a1"}, "a 1 A\nb 2\nc 3 C\n"},
		{[]string{"-e", "X", "-o", "0,1.2,2.2", "-a1", "-a2", "f1", "f2"}, "a 1 A\nb 2 X\nc 3 C\nd X D\n"},
		{[]string{"-o", "0 2.2", "f1", "f2"}, "a A\nc C\n"},
		{[]string{"-o", "1.2", "-o", "2.2", "f1", "f2"}, "A\nC\n"},
		{[]string{"-e", "A", "-e", "B", "-o", "1.2,2.3", "f1", "f2"}, "1 B\n3 B\n"},
		{[]string{"-a1", "-o", "1.1,2.1", "f1", "f2"}, "a a\nb \nc c\n"},
		{[]string{"-a2", "-e", "Z", "-o", "0,1.1,2.1", "f1", "f2"}, "a a a\nc c c\nd Z d\n"},
		{[]string{"-o", "0", "-a1", "-a2", "f1", "f2"}, "a\nb\nc\nd\n"},
		{[]string{"-o", ",", "f1", "f2"}, "\n\n"},
		{[]string{"-2", "3", "f1", "f2"}, ""},
		{[]string{"-t:", "t1", "t2"}, "k:1:x:2\nm::y:3\n"},
		{[]string{"-t:", "-o", "1.3,2.2", "t1", "t2"}, "x:2\ny:3\n"},
		{[]string{"-t:", "-e", "E", "t1", "t2"}, "k:1:x:2\nm:E:y:3\n"},
		{[]string{"-t:", "-e", "E", "e1", "e2"}, "k:E:x:E\n"},
		{[]string{"-t", " ", "s1", "s2"}, "a  1 x 2\n"},
		{[]string{"s1", "s2"}, "a 1 x 2\n"},
		{[]string{"m1", "m2"}, "a 1 A\na 1 B\na 1 C\na 2 A\na 2 B\na 2 C\n"},
		{[]string{"u1", "u2"}, "b 1 B\n"},
		{[]string{"-e", "E", "b1", "b2"}, "E\na 1 A\n"},
		{[]string{"r1", "r2"}, "a 1 A\nb 2 B\n"},
		{[]string{"c1", "c2"}, "B 1 x\na 2 y\n"},
		{[]string{"-", "f2"}, "a 9 A\n"},
	} {
		if got, _, err := runSmall(t, dir, "a 9\n", "join", test.args...); err != nil || got != test.want {
			t.Errorf("join %q = %q, %v; want %q", test.args, got, err, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-a1", "-v2"}, "-a and -v are exclusive"},
		{[]string{"-a", "3"}, "-a and -v take either 1 or 2"},
		{[]string{"-a", "12"}, "-a and -v take either 1 or 2"},
		{[]string{"-t", "::"}, "separators are single characters"},
		{[]string{"-t", ""}, "separators are single characters"},
		{[]string{"-1", "0"}, "field 0 does not exist"},
		{[]string{"-1", "x"}, "invalid number 'x'"},
		{[]string{"-2", "3000000000"}, "number 3000000000 is not in 0..2147483647 range"},
		{[]string{"-o", "3.1"}, "field specifier must be 0, 1.x or 2.x"},
		{[]string{"-o", "00"}, "field specifier must be 0, 1.x or 2.x"},
		{[]string{"-o", "1."}, "field specifier must be 0, 1.x or 2.x"},
		{[]string{"-o", "1.0"}, "field number cannot be 0"},
		{[]string{"-o", "1.x"}, "invalid number 'x'"},
		{[]string{"-o", "1.123456789012345678901"}, "field specifier too large"},
	} {
		args := append(test.args, "f1", "f2")
		if _, _, err := runSmall(t, dir, "", "join", args...); err == nil || err.Error() != test.want {
			t.Errorf("join %q: %v, want %q", args, err, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-", "-"}, "cannot combine stdin with itself"},
		{[]string{"f1"}, "missing operand"},
		{[]string{"f1", "f2", "f3"}, "extra operand 'f3'"},
	} {
		if _, _, err := runSmall(t, dir, "", "join", test.args...); err == nil || err.Error() != test.want {
			t.Errorf("join %q: %v, want %q", test.args, err, test.want)
		}
	}
}
