package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a function whose name has an expansion or quoting in it, `$foo-bar() {...}` or
// `foo-$(echo hi)() {...}`, is refused when the definition runs, as bash refuses it: the name as
// written, not a valid identifier, status 1, nothing defined and the script going on; under
// set -o posix the script ends, 2. nemosh and sh keep refusing it as the script is read, a syntax
// error, as Oils has ash do. Measured 2026-11-22 with Git for Windows' bash 5.3; Oils'
// func-parsing.test.sh.
func TestFunctionName_bashModeRefusedWhenDefined(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
		status             int
		bash               bool
	}{
		{code: "echo before\n$foo-bar() { ls ; }; echo after=$?", stdout: "before\nafter=1\n", said: "line 2: `$foo-bar': not a valid identifier\n", bash: true},
		{code: "foo-$(echo hi)() { ls ; } || echo or=$?", stdout: "or=1\n", said: "`foo-$(echo hi)': not a valid identifier\n", bash: true},
		{code: "x=1; foo$x() {\n  echo hi\n}\necho st=$?; type foo1 >/dev/null 2>&1 || echo none", stdout: "st=1\nnone\n", said: "line 3: `foo$x': not a valid identifier\n", bash: true},
		{code: "a\"b\"() { :; }; echo st=$?", stdout: "st=1\n", said: "`a\"b\"': not a valid identifier\n", bash: true},
		{code: "set -o posix\n$foo-bar() { ls ; }; echo st=$?", said: "`$foo-bar': not a valid identifier\n", status: 2, bash: true},
		{code: "foo-bar() { echo ok; }; foo-bar", stdout: "ok\n", bash: true},
		{code: "echo before\n$foo-bar() { ls ; }", said: "unexpected )\n", status: 2},
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
