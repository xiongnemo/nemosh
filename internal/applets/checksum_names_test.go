package applets_test

import "testing"

// sum, cksum and crc32 name what they read as busybox's do: a `-` named is `-`, and only stdin
// read for want of an operand goes without a name. sum's BSD line keeps the blank before the name
// it leaves out, System V's names stdin `-`, and -r wins over -s whichever comes first. A `-` was
// never named, the blank was dropped as GNU drops it, and the later of -r and -s won. Each answer
// is busybox-w32's, measured.
func TestChecksum_namesWhatItReadAsBusyboxDoes(t *testing.T) {
	dir := checksumFixture(t)
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{"sum", nil, "36979     1 \n"},
		{"sum", []string{"-"}, "36979     1 \n"},
		{"sum", []string{"h.txt"}, "36979     1 \n"},
		{"sum", []string{"-", "h.txt"}, "36979     1 -\n36979     1 h.txt\n"},
		{"sum", []string{"-s"}, "542 1 -\n"},
		{"sum", []string{"-s", "-"}, "542 1 -\n"},
		{"sum", []string{"-r", "-s", "h.txt"}, "36979     1 \n"},
		{"sum", []string{"-s", "-r", "h.txt"}, "36979     1 \n"},
		{"cksum", nil, "3015617425 6\n"},
		{"cksum", []string{"-"}, "3015617425 6 -\n"},
		{"cksum", []string{"-", "h.txt"}, "3015617425 6 -\n3015617425 6 h.txt\n"},
		{"crc32", []string{"-"}, "363a3020 -\n"},
	} {
		if stdout, _, err := runChecksum(t, dir, test.applet, "hello\n", test.args...); err != nil || stdout != test.want {
			t.Errorf("%s %q = %q, %v; want %q", test.applet, test.args, stdout, err, test.want)
		}
	}
}
