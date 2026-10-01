package applets

import "testing"

// With ibs and obs two sizes, dd gathers what it reads into blocks of obs, writes each when it
// is full and the rest at the end, as busybox's dd does with its second buffer. bs= sets both,
// so `bs=4 obs=8` gathers and `obs=8 bs=4` does not. Each answer is busybox-w32's, measured;
// every record went out as it came in, so `ibs=4 obs=8 count=2` said "0+2 records out".
func TestDd_gathersBlocksOfObs(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		args          []string
		stdin, stdout string
		counts        string
	}{
		{args: []string{"ibs=4", "obs=8", "count=2"}, stdin: "abcdefghij", stdout: "abcdefgh", counts: "2+0 records in\n1+0 records out\n"},
		{args: []string{"ibs=3", "obs=4"}, stdin: "abcdefghij", stdout: "abcdefghij", counts: "3+1 records in\n2+1 records out\n"},
		{args: []string{"ibs=8", "obs=3"}, stdin: "abcdefghij", stdout: "abcdefghij", counts: "1+1 records in\n3+1 records out\n"},
		{args: []string{"ibs=4", "obs=8", "conv=sync"}, stdin: "abcdef", stdout: "abcdef\x00\x00", counts: "1+1 records in\n1+0 records out\n"},
		{args: []string{"ibs=2", "obs=8", "skip=1"}, stdin: "abcdefghij", stdout: "cdefghij", counts: "4+0 records in\n1+0 records out\n"},
		{args: []string{"bs=4", "obs=8"}, stdin: "abcdefghij", stdout: "abcdefghij", counts: "2+1 records in\n1+1 records out\n"},
		{args: []string{"obs=8", "bs=4"}, stdin: "abcdefghij", stdout: "abcdefghij", counts: "2+1 records in\n2+1 records out\n"},
		{args: []string{"ibs=4", "obs=4"}, stdin: "abcdefghij", stdout: "abcdefghij", counts: "2+1 records in\n2+1 records out\n"},
	} {
		stdout, stderr, status := runApplet(t, "dd", test.args, test.stdin)
		if stdout != test.stdout || stderr != test.counts || status != 0 {
			t.Errorf("dd %q = %q, %q, %d; want %q, %q", test.args, stdout, stderr, status, test.stdout, test.counts)
		}
	}
}
