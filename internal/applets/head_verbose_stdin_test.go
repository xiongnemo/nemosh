package applets_test

import "testing"

// head -v heads standard input when no FILE is named, as "standard input", the name it gives
// `-`, as busybox's head and GNU's do; the last of -q and -v decides. Each answer is
// busybox-w32's, measured; there was no header.
func TestHead_verboseHeadsStandardInput(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-v", "-n1"}, want: "==> standard input <==\na\n"},
		{args: []string{"-vc1"}, want: "==> standard input <==\na"},
		{args: []string{"-q", "-v", "-n1"}, want: "==> standard input <==\na\n"},
		{args: []string{"-v", "-q", "-n1"}, want: "a\n"},
		{args: []string{"-n1"}, want: "a\n"},
	} {
		if got, _, err := runSmall(t, dir, "a\nb\n", "head", test.args...); got != test.want || err != nil {
			t.Errorf("head %q = %q, %v; want %q", test.args, got, err, test.want)
		}
	}
}
