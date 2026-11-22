package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a for or select loop whose variable is no name is refused when it runs, as bash
// refuses it: the name as written, said to be not a valid identifier, quotes and all for one
// quoted or expanded, status 1, the loop not run and the script going on; under set -o posix the
// script ends, 2. busybox refuses an unquoted one as the script is read, a syntax error that
// runs none of it, which nemosh and sh keep. Measured 2026-11-22 with Git for Windows' bash 5.3;
// Oils' loop.test.sh and parse-errors.test.sh.
func TestLoopName_bashModeRefusedWhenItRuns(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
		status             int
		bash               bool
	}{
		{code: "echo before\nfor i.j in a b; do echo in; done; echo st=$?", stdout: "before\nst=1\n", said: "line 2: `i.j': not a valid identifier\n", bash: true},
		{code: "for - in a; do echo in; done\necho st=$?", stdout: "st=1\n", said: "`-': not a valid identifier\n", bash: true},
		{code: "for \"q\" in a; do echo in; done; echo st=$?", stdout: "st=1\n", said: "`\"q\"': not a valid identifier\n", bash: true},
		{code: "x=y; for $x in a; do echo in; done; echo st=$?", stdout: "st=1\n", said: "`$x': not a valid identifier\n", bash: true},
		{code: "for 1x; do echo in; done; echo st=$?", stdout: "st=1\n", said: "`1x': not a valid identifier\n", bash: true},
		{code: "select a.b in x; do break; done; echo st=$?", stdout: "st=1\n", said: "`a.b': not a valid identifier\n", bash: true},
		{code: "set -o posix\nfor i.j in a; do echo in; done; echo st=$?", said: "`i.j': not a valid identifier\n", status: 2, bash: true},
		{code: "for i in a b; do echo $i; done", stdout: "a\nb\n", bash: true},
		{code: "echo before\nfor i.j in a b; do echo in; done", said: "bad for loop variable i.j\n", status: 2},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != test.status || stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want %d, %q and %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.status, test.stdout, test.said)
		}
	}
}
