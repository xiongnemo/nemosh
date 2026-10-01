package applets_test

import (
	"strings"
	"testing"
)

// addr,+N selects the line the address matches and the N after it, each time the address
// matches, as busybox and GNU read it; a plus with no digits is no address. Each answer is
// busybox-w32's, measured; `,+N` was refused as no address after the comma.
func TestSed_rangeOfTheLinesFollowing(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "banana 3\napple 10\ncherry 2\napple 10\nDate 7\n\nelder 1\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"/apple/,+1d", "a.txt"}, want: "banana 3\n\nelder 1\n"},
		{args: []string{"-n", "/apple/,+1p", "a.txt"}, want: "apple 10\ncherry 2\napple 10\nDate 7\n"},
		{args: []string{"-n", "2,+2p", "a.txt"}, want: "apple 10\ncherry 2\napple 10\n"},
		{args: []string{"-n", "/banana/,+0p", "a.txt"}, want: "banana 3\n"},
	} {
		if got, stderr, err := runSmall(t, dir, "", "sed", test.args...); got != test.want || err != nil {
			t.Errorf("sed %q = %q, %v (%s); want %q", test.args, got, err, stderr, test.want)
		}
	}
	if _, _, err := runSmall(t, dir, "", "sed", "2,+p", "a.txt"); err == nil || !strings.Contains(err.Error(), "no address after comma") {
		t.Errorf("sed 2,+p = %v; want no address after comma", err)
	}
}
