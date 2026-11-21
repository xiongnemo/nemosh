package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a compound command's redirection that fails runs the ERR trap and, under set -e,
// ends the script, status 1, as bash 5.2 and later have it: `{ cat; } < nofile`, a loop's, an
// if's and a case's. In a condition, or before && or ||, it does neither. busybox goes on, and
// runs no ERR trap for it, as nemosh and sh do. Measured 2026-11-22, a script per case, with Git
// for Windows' bash 5.3 and busybox-w32; Oils' errexit.test.sh.
func TestCompoundRedirection_bashModeFailureIsErrexits(t *testing.T) {
	for _, test := range []struct {
		code, stdout string
		status       int
		bash         bool
	}{
		{code: "set -e\n{ cat; } < nonexist\necho status=$?", status: 1, bash: true},
		{code: "set -e\nwhile read line; do echo $line; done < nonexist\necho status=$?", status: 1, bash: true},
		{code: "set -e\nif true; then echo in; fi < nonexist\necho after", status: 1, bash: true},
		{code: "set -e\ncase x in x) echo in;; esac < nonexist\necho after", status: 1, bash: true},
		{code: "trap 'echo ERR' ERR\n{ cat; } < nonexist\necho after=$?", stdout: "ERR\nafter=1\n", bash: true},
		{code: "trap 'echo ERR' ERR\nwhile false; do :; done < nonexist\necho after=$?", stdout: "ERR\nafter=1\n", bash: true},
		{code: "set -e\nif { cat; } < nonexist; then echo yes; else echo no; fi\necho after", stdout: "no\nafter\n", bash: true},
		{code: "set -e\n{ cat; } < nonexist || echo handled\necho after", stdout: "handled\nafter\n", bash: true},
		// As nemosh, busybox's.
		{code: "set -e\n{ cat; } < nonexist\necho status=$?", stdout: "status=1\n"},
		{code: "trap 'echo ERR' ERR\n{ cat; } < nonexist\necho after=$?", stdout: "after=1\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), "cd \""+t.TempDir()+"\"\n"+test.code+"\n")
		if status != test.status || stdout.String() != test.stdout {
			t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want %d and %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.status, test.stdout)
		}
	}
}
