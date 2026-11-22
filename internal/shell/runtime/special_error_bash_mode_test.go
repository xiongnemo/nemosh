package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A special builtin's redirection that fails ends a busybox script, and a bash one only in bash's
// POSIX mode: outside it the builtin answers 1 and the script goes on, `: > /no/dir/f`, export's,
// set's, exec's, `.`'s, eval's and readonly's alike. And `command` in front of one contains no
// error bash ends the shell for: under set -u, `command eval 'echo $nope'` ends a bash script as
// the eval alone does, where busybox's command keeps the script going. Bash mode takes bash's,
// busybox's under set -o posix; nemosh and sh keep busybox's. Measured 2026-11-22, a script per
// case, with Git for Windows' bash 5.3 and busybox-w32.
func TestSpecialError_bashModeGoesOnAsBash(t *testing.T) {
	const redirections = ": > /no/such/dir/f; echo a1=$?\nexport q > /no/such/dir/f; echo a2=$?\nset > /no/such/dir/f; echo a3=$?\n" +
		"exec 3> /no/such/dir/f; echo a4=$?\n. /dev/null > /no/such/dir/f; echo a5=$?\neval : > /no/such/dir/f; echo a6=$?\n" +
		"readonly r=1 > /no/such/dir/f; echo a7=$?\necho end\n"
	for _, test := range []struct {
		name, script, want string
		bash               bool
		status             int
	}{
		{name: "redirections", script: redirections, bash: true, want: "a1=1\na2=1\na3=1\na4=1\na5=1\na6=1\na7=1\nend\n"},
		{name: "redirections, posix", script: "set -o posix\n" + redirections, bash: true, status: 1},
		{name: "redirections as nemosh", script: redirections, status: 1},
		{name: "command eval", script: "set -u\ncommand eval 'echo in; echo $nope'; echo out=$?\necho end\n", bash: true, want: "in\n", status: 1},
		{name: "command eval as nemosh", script: "set -u\ncommand eval 'echo in; echo $nope'; echo out=$?\necho end\n", want: "in\nout=2\nend\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), test.script)
		if stdout.String() != test.want || status != test.status {
			t.Errorf("%s: status %d, stdout %q, stderr %q; want %d and %q", test.name, status, stdout.String(), stderr.String(), test.status, test.want)
		}
	}
}
