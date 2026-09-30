package applets_test

import "testing"

// Every `-` is the one standard input, read a line to a column in turn, as busybox's paste reads
// it, and under -s an operand with no lines writes none. `seq 5 | paste - -` was each line alone
// with a tab after it, and `paste -s` wrote an empty line for an empty operand. Each answer is
// busybox-w32's, measured.
func TestPaste_readsARepeatedDashALineAtATime(t *testing.T) {
	for _, test := range []struct {
		input string
		args  []string
		want  string
	}{
		{"1\n2\n3\n4\n5\n", []string{"-", "-"}, "1\t2\n3\t4\n5\t\n"},
		{"1\n2\n3\n4\n5\n6\n", []string{"-", "-", "-"}, "1\t2\t3\n4\t5\t6\n"},
		{"1\n2\n3\n", []string{"-d,", "-", "-"}, "1,2\n3,\n"},
		{"1\n2\n3\n4\n", []string{"-s", "-", "-"}, "1\t2\t3\t4\n"},
		{"", []string{"-", "-"}, ""},
	} {
		stdout, _, err := runAppletWithInput(t, test.input, "paste", test.args...)
		if err != nil || stdout != test.want {
			t.Errorf("paste %q over %q = %q (err %v), want %q", test.args, test.input, stdout, err, test.want)
		}
	}
}
