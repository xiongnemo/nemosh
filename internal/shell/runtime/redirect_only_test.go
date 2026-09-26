package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A command with no command name still performs its redirections, as POSIX 2.9.1 says and
// both references do: `> file` creates or empties the file -- the everyday way to -- and
// `< missing` fails with status 1. Here the redirections of such a command were dropped, so
// `> file` did nothing at all and `< missing` succeeded. With assignments and no name, a
// redirection that fails leaves the assignments undone, as busybox-w32 has it; bash assigns
// first. And with no name and no assignment the status is the last command substitution's.
func TestRuntime_commandWithNoNamePerformsItsRedirections(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	tests := []struct {
		script, want string
		status       int
	}{
		{`> "` + dir + `/made"; test -e "` + dir + `/made" && echo created`, "created\n", 0},
		{`printf full > "` + dir + `/t"; > "` + dir + `/t"; wc -c < "` + dir + `/t"`, "0\n", 0},
		{`< /no/such/file 2>/dev/null; echo "status=$?"`, "status=1\n", 0},
		{`y=2 > "` + dir + `/assigned"; echo "[$y]"; test -e "` + dir + `/assigned" && echo created`, "[2]\ncreated\n", 0},
		{`x=1 < /no/such/file 2>/dev/null; echo "[$x] $?"`, "[] 1\n", 0},
		{`$(false); echo "status=$?"`, "status=1\n", 0},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d", stdout, status, test.want, test.status)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(dir, "made")); err != nil {
		t.Errorf("> file left no file: %v", err)
	}
}
