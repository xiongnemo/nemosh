package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// flock, busybox's, a builtin here because its FD form locks a descriptor the shell holds and
// its FILE form runs a command. A second flock of a held file waits, or under -n answers 1 and
// says nothing; the lock keeps out other flocks and nothing else, so the command can read the
// file it holds, which busybox-w32's cannot; and an empty lock file excludes too, which
// busybox-w32's does not. It was not found.
func TestFlock_locksAgainstAnotherFlock(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := strings.Join([]string{
		`d='` + dir + `'`,
		`flock -n "$d/l" -c 'flock -n "$d/l" echo inner; echo "inner $?"'`,
		`flock -n "$d/l" echo free; echo "free $?"`,
		`flock "$d/l" -c 'exit 3'; echo "c $?"`,
		`printf 'data\n' > "$d/f"; flock "$d/f" cat "$d/f"`,
		`exec 9>"$d/fd"; flock -n 9; echo "fd $?"`,
		`flock -n "$d/fd" true; echo "fd held $?"`,
		`flock -u 9; flock -n "$d/fd" true; echo "after -u $?"`,
		`flock -s "$d/s" -c 'flock -sn "$d/s" echo shared; flock -n "$d/s" true; echo "exclusive $?"'`,
		`exec 9>&-`,
	}, "\n") + "\n"
	status, stdout, stderr := runSetScript(t, script)
	want := "inner 1\nfree\nfree 0\nc 3\ndata\nfd 0\nfd held 1\nafter -u 0\nshared\nexclusive 1\n"
	if status != 0 || stdout != want || stderr != "" {
		t.Fatalf("got %d, %q, %q; want %q", status, stdout, stderr, want)
	}
}

// What flock refuses, it says, status 1: no operand, -c with more than one word, a number that
// names no file the shell holds. FILE is made before -c is looked at, as busybox makes it, so
// the script runs in a directory of its own.
func TestFlock_refusals(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	for _, test := range []struct {
		script, says string
	}{
		{script: "flock", says: "flock: expected a descriptor"},
		{script: "flock -z f true", says: "flock: unknown option -- z"},
		{script: "flock f -c a b", says: "flock: -c takes only one argument"},
		{script: "flock 7", says: "flock: 7: Bad file descriptor"},
	} {
		status, _, stderr := runSetScript(t, "cd '"+dir+"'\n"+test.script+"\n")
		if status != 1 || !strings.Contains(stderr, test.says) {
			t.Errorf("%s = %d, %q; want 1 and %q", test.script, status, stderr, test.says)
		}
	}
}
