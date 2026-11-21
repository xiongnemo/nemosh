package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An operand that is no variable name ends a busybox script, status 2: `export 1x=2`, `export:
// 1x: bad variable name`. bash says "`1x=2': not a valid identifier", quoting the operand as
// written, answers 1 once every other operand has been dealt with, and goes on; `unset 1x`,
// which may name a function, is no error at all, and `unset -v 1x` is one. Bash mode takes
// bash's, and nemosh and sh keep busybox's. Measured 2026-11-21, a script per case, with Git
// for Windows' bash 5.3 and busybox-w32.
func TestBadName_bashModeRefusesAsBashDoes(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
	}{
		{code: "export 1x=2; echo same=$?", stdout: "same=1\nnext=0\n", said: "line 1: export: `1x=2': not a valid identifier"},
		{code: "export 1x a=1; echo same=$? a=$a; printenv a", stdout: "same=1 a=1\n1\nnext=0\n", said: "export: `1x': not a valid identifier"},
		{code: "readonly 1x=2; echo same=$?", stdout: "same=1\nnext=0\n", said: "readonly: `1x=2': not a valid identifier"},
		{code: "f() { local 1x=2; echo in-f=$?; }; f; echo same=$?", stdout: "in-f=1\nsame=0\nnext=0\n", said: "local: `1x=2': not a valid identifier"},
		{code: "unset -v 1x; echo same=$?", stdout: "same=1\nnext=0\n", said: "unset: `1x': not a valid identifier"},
		{code: "unset 1x; echo same=$?", stdout: "same=0\nnext=0\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"\necho next=$?\n")
		said := stderr.String()
		if status != 0 || stdout.String() != test.stdout || !strings.Contains(said, test.said) || test.said == "" && said != "" {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want 0, %q and %q", test.code, status, stdout.String(), said, test.stdout, test.said)
		}
	}
	// declare is bash's under every name, busybox having none: the operand as written, and the
	// others declared all the same. It said `declare: 1x: not a valid name` and stopped there.
	for _, bash := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "declare 1x=2 b=3; echo st=$? b=$b\n")
		if stdout.String() != "st=1 b=3\n" || !strings.Contains(stderr.String(), "declare: `1x=2': not a valid identifier") {
			t.Errorf("declare, bash mode %v: stdout %q, stderr %q", bash, stdout.String(), stderr.String())
		}
	}
	// As nemosh, busybox's: the script ends there, status 2.
	for _, code := range []string{"export 1x=2", "readonly 1x=2", "unset 1x", "unset -v 1x"} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), code+"\necho next=$?\n")
		if status != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "1x: bad variable name") {
			t.Errorf("as nemosh %q: status %d, stdout %q, stderr %q; want busybox's end, 2", code, status, stdout.String(), stderr.String())
		}
	}
}
