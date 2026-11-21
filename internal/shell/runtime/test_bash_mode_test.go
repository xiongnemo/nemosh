package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// test and [ in bash mode read their arguments as bash 5.3's test.c does: by how many there
// are, POSIX's rules with -a and -o between two words their AND and OR, and bash's grammar past
// four, unary -a being -e; and a refusal is said in bash's words, after the shell's prefix, status
// 2. busybox refuses `[ -a / ]` and `[ -z -a ] ]`, and says `bad number` and `unknown operand`,
// which nemosh and sh keep. Measured 2026-11-22, each line of a script, with Git for Windows'
// bash 5.3; Oils' builtin-bracket.test.sh.
func TestTest_bashModeReadsArgumentsAsBash(t *testing.T) {
	for _, test := range []struct {
		code, said string
		status     int
	}{
		{code: "[ -a / ]"},
		{code: "[ -a /nonexistent ]", status: 1},
		{code: "[ -z -a ] ]"},
		{code: "[ -z -a -a ]"},
		{code: "[ -f -a -d ]"},
		{code: "[ x -o '' ]"},
		{code: "[ '' -a x ]", status: 1},
		{code: "[ a -eq b ]", said: "line 1: [: a: integer expected\n", status: 2},
		{code: "[ '' -eq 1 ]", said: "[: : integer expected\n", status: 2},
		{code: "[ 12345678901234567890 -eq 1 ]", said: "[: 12345678901234567890: integer expected\n", status: 2},
		{code: "[ ' 12 ' -eq 12 ]"},
		{code: "[ -t invalid ]", said: "[: invalid: integer expected\n", status: 2},
		{code: "[ x y ]", said: "[: x: unary operator expected\n", status: 2},
		{code: "test -q x", said: "test: -q: unary operator expected\n", status: 2},
		{code: "[ '(' foo ]", said: "[: (: unary operator expected\n", status: 2},
		{code: "[ x y z ]", said: "[: y: binary operator expected\n", status: 2},
		{code: "[ a b c d e ]", said: "[: too many arguments\n", status: 2},
		{code: "[ 1 -eq 1 x ]", said: "[: too many arguments\n", status: 2},
		{code: "[ 1 -eq 1 -a ]", said: "[: argument expected\n", status: 2},
		{code: "[ '(' 1 -eq 1 ]", said: "[: `)' expected, found ]\n", status: 2},
		{code: "test '(' 1 -eq 1", said: "test: `)' expected\n", status: 2},
		{code: "[ 1 -eq 1", said: "line 1: [: missing `]'\n", status: 2},
		{code: "[ 1 -eq 1 -a 2 -eq 2 ]"},
		{code: "[ '(' 1 -eq 1 ')' ]"},
		{code: "[ '(' x ')' -a ')' ]"},
		{code: "[ x -a y -a z w ]", said: "[: too many arguments\n", status: 2},
		{code: "[ ! '(' x ')' ]", status: 1},
		{code: "[ ! -e ]", status: 1},
		{code: "[ ! ! x ]"},
		{code: "[ ! ]"},
		{code: "[ '(' ]"},
		{code: "test", status: 1},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.RunScript(context.Background(), test.code+"\necho st=$?\n")
		want := "st=" + string(rune('0'+test.status)) + "\n"
		if stdout.String() != want || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("%s: stdout %q, stderr %q; want %q and %q", test.code, stdout.String(), stderr.String(), want, test.said)
		}
	}
	// As nemosh, busybox's.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), "[ -a / ]\necho st=$?\n[ a -eq b ]\necho st=$?\n")
	if stdout.String() != "st=2\nst=2\n" || !strings.Contains(stderr.String(), "[: a: bad number\n") {
		t.Errorf("as nemosh: stdout %q, stderr %q; want busybox's", stdout.String(), stderr.String())
	}
}
