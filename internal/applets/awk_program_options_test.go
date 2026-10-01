package applets_test

import (
	"os"
	"path/filepath"
	"testing"
)

// -e is program text and -f a file, read in the order given as one program; -E is -f that
// ends the options, so what follows is ARGV; -W is said to be ignored. Each answer is
// busybox-w32's, measured, and each option was refused as invalid.
func TestAwk_takesBusyboxsProgramOptions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.awk"), []byte("BEGIN { print \"f\" }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args           []string
		stdout, stderr string
	}{
		{args: []string{"-e", "BEGIN { print 1 }", "-e", "BEGIN { print 2 }"}, stdout: "1\n2\n"},
		{args: []string{"-f", "p.awk", "-e", "BEGIN { print \"e\" }"}, stdout: "f\ne\n"},
		{args: []string{"-e", "function g() { return \"g\" }", "-e", "BEGIN { print g() }"}, stdout: "g\n"},
		{args: []string{"-E", "p.awk", "-v", "x=1"}, stdout: "f\n"},
		{args: []string{"-E", "p.awk", "-e", "BEGIN { print ARGV[1] }"}, stdout: "f\n"},
		{args: []string{"-W", "posix", "BEGIN { print 3 }"}, stdout: "3\n", stderr: "awk: -W is ignored\n"},
	} {
		stdout, stderr, err := runSmall(t, dir, "", "awk", test.args...)
		if stdout != test.stdout || stderr != test.stderr || err != nil {
			t.Errorf("awk %q = %q, %q, %v; want %q, %q", test.args, stdout, stderr, err, test.stdout, test.stderr)
		}
	}
}
