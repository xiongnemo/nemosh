package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An expansion that fails ends a busybox script, status 2. bash, outside POSIX mode, abandons the
// rest of the line, status 1, and goes on with the next: an arithmetic error even under `set -e`,
// a bad substitution and `${1:=x}` unless `set -e` ends the script there. `${x?}` and an unset
// name under `set -u` end it in bash too, status 1. Bash mode takes bash's, and nemosh and sh
// keep busybox's. Measured 2026-11-21, a script per case, with Git for Windows' bash 5.3.
func TestExpansionError_bashModeAsBashHasIt(t *testing.T) {
	for _, test := range []struct {
		code, stdout, said string
		status             int
	}{
		{code: "echo ${1:=x}", stdout: "next=1\n", said: "line 1: $1: cannot assign in this way"},
		{code: "echo $((1/0))", stdout: "next=1\n"},
		{code: "echo $((1+))", stdout: "next=1\n"},
		{code: "v=$((1/0))", stdout: "next=1\n"},
		{code: "f() { echo $((1/0)); echo in-f; }; f", stdout: "next=1\n"},
		{code: "echo ${x!y}", stdout: "next=1\n", said: "line 1: ${x!y}: bad substitution"},
		{code: "set -e; echo $((1/0))", stdout: "next=1\n"},
		{code: "set -e; echo ${x!y}", status: 1, said: "${x!y}: bad substitution"},
		{code: "set -u; echo $nosuch", status: 1, said: "line 1: nosuch: unbound variable"},
		{code: "echo ${nosuch?gone}", status: 1, said: "line 1: nosuch: gone"},
		{code: "v=$(echo ${nosuch?gone}); echo sub=$?", stdout: "sub=1\nsame=0\nnext=0\n", said: "nosuch: gone"},
		// A case pattern's error is the case command's, and abandons it: its line was the next
		// command's, which went with it. Oils' fatal-errors.test.sh.
		{code: "case foo in ( $((42/0)) ) echo hi ;; esac\necho inside=$?", stdout: "inside=1\nsame=0\nnext=0\n"},
		// A completion function's error fails compgen, status 1, its candidates and all.
		// Oils' builtin-completion.test.sh.
		{code: "_foo() { COMPREPLY=( foo bar ); COMPREPLY+=( $((1/0)) ); }; compgen -F _foo foo 2>/dev/null", stdout: "next=1\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"; echo same=$?\necho next=$?\n")
		if status != test.status || stdout.String() != test.stdout || !strings.Contains(stderr.String(), test.said) {
			t.Errorf("%q: status %d, stdout %q, stderr %q; want %d, %q and %q", test.code, status, stdout.String(), stderr.String(), test.status, test.stdout, test.said)
		}
	}
	// As nemosh, busybox's: the script ends, status 2.
	for _, code := range []string{"echo $((1/0))", "echo ${x!y}", "echo ${nosuch?gone}", "set -u; echo $nosuch", "echo ${1:=x}"} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if status := rt.RunScript(context.Background(), code+"; echo same=$?\necho next=$?\n"); status != 2 || stdout.Len() != 0 {
			t.Errorf("as nemosh %q: status %d, stdout %q, stderr %q; want busybox's end, 2", code, status, stdout.String(), stderr.String())
		}
	}
}
