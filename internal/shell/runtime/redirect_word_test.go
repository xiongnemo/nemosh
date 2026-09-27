package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A redirection's word is expanded as busybox expands it, as POSIX has it: parameters,
// command substitutions, arithmetic, a tilde and quote removal, and no field splitting, no
// pathname expansion and no brace expansion. A word that came to more than one field was
// bash's "ambiguous redirect" here and the command did not run: `> $x` with a blank in x --
// on Windows, `C:/Program Files` -- and every Windows path once IFS held a colon. The answers
// are busybox-w32's, each measured in an empty directory.
func TestRuntime_redirectionWordIsOneWord(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{"x='sp ace'; echo one > $x; cat 'sp ace'", "one\n", 0},
		{"set -- a b; echo two > \"$@\"; cat 'a b'", "two\n", 0},
		{"echo four > {p,q}; cat '{p,q}'", "four\n", 0},
		{"IFS=,; y='m,n'; echo five > $y; IFS=' '; cat 'm,n'", "five\n", 0},
		{"x=; echo six > $x; echo st=$?", "st=1\n", 0},
		{"x='sp ace'; echo seven >> $x; exec 3> $x; echo eight >&3; exec 3>&-; cat \"$x\"", "eight\n", 0},
		{"x='sp ace'; echo nine > \"$x\"; cat < $x", "nine\n", 0},
		{"set -- first second; echo ran >$@ >probe; cat probe; ls", "ran\nfirst second\nprobe\n", 0},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			script := fmt.Sprintf("cd '%s'\n%s\n", filepath.ToSlash(t.TempDir()), test.script)
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as busybox-w32 answers", stdout, status, test.want, test.status)
			}
		})
	}
}
