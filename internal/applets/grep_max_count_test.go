package applets_test

import (
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// -m stops at its count and grep still answers as it does without it: -c counts what was read
// and -l names the file, where both said nothing. The trailing context ends at the next line
// that would have been selected, as busybox's and GNU's do; it went on through it. Each answer
// is busybox-w32's and GNU grep 3.0's, which agree.
func TestGrep_maxCountStillCountsAndNames(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"f": "banana 3\napple 10\ncherry 2\napple 10\nDate 7\n\nelder 1\n"})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-c", "-m2", "a", "f"}, want: "2\n"},
		{args: []string{"-c", "-m2", "a", "f", "f"}, want: "f:2\nf:2\n"},
		{args: []string{"-cv", "-m1", "a", "f"}, want: "1\n"},
		{args: []string{"-l", "-m1", "a", "f"}, want: "f\n"},
		{args: []string{"-nA2", "-m1", "a", "f"}, want: "1:banana 3\n"},
		{args: []string{"-nA1", "-m1", "e", "f"}, want: "2:apple 10\n"},
		{args: []string{"-nA1", "-m2", "an", "f"}, want: "1:banana 3\n2-apple 10\n"},
		{args: []string{"-vnA1", "-m1", "a", "f"}, want: "3:cherry 2\n4-apple 10\n"},
	} {
		if got, _, err := runSmall(t, dir, "", "grep", test.args...); got != test.want || err != nil {
			t.Errorf("grep %q = %q, %v; want %q", test.args, got, err, test.want)
		}
	}
}

// -m0 reads no line and prints nothing, and the status is 1, as GNU's grep has it. busybox's
// prints no line either but counts the line it stopped at: its -c says 1, and its status is 0.
// -m0 printed every match.
func TestGrep_maxCountZeroSelectsNothing(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"f": "banana 3\napple 10\n"})
	for _, args := range [][]string{
		{"-m0", "a", "f"},
		{"-c", "-m0", "a", "f"},
		{"-l", "-m0", "a", "f"},
		{"-n", "-A1", "-m0", "a", "f"},
	} {
		if got, _, err := runSmall(t, dir, "", "grep", args...); got != "" || !errors.Is(err, applets.ErrExitFalse) {
			t.Errorf("grep %q = %q, %v; want nothing and status 1", args, got, err)
		}
	}
}
