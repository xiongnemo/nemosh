package applets_test

import (
	"strings"
	"testing"
)

// `tail +3` is from the third line on, as busybox's tail reads a first argument of + and a
// digit, and only the first: anywhere else it is a FILE, as busybox opens one. Each answer is
// busybox-w32's, measured; +3 was opened as a FILE.
func TestTail_takesPlusNAsItsFirstArgument(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"five": "1\n2\n3\n4\n5\n"})
	for _, test := range []struct {
		args   []string
		stdin  string
		stdout string
		fails  string
	}{
		{args: []string{"+3", "five"}, stdout: "3\n4\n5\n"},
		{args: []string{"+0", "five"}, stdout: "1\n2\n3\n4\n5\n"},
		{args: []string{"+1k", "five"}, stdout: ""},
		{args: []string{"+2"}, stdin: "a\nb\nc\n", stdout: "b\nc\n"},
		{args: []string{"+3", "five", "five"}, stdout: "==> five <==\n3\n4\n5\n\n==> five <==\n3\n4\n5\n"},
		{args: []string{"+3x", "five"}, fails: "invalid number '3x'"},
		{args: []string{"-n", "1", "+3", "five"}, stdout: "5\n", fails: "+3"},
	} {
		stdout, stderr, err := runSmall(t, dir, test.stdin, "tail", test.args...)
		failed := err != nil && strings.Contains(stderr+err.Error(), test.fails)
		if stdout != test.stdout || (test.fails == "") != (err == nil) || test.fails != "" && !failed {
			t.Errorf("tail %q = %q, %q, %v; want %q, failing with %q", test.args, stdout, stderr, err, test.stdout, test.fails)
		}
	}
}
