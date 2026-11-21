package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Bash mode's read refuses what it cannot take as bash's does: a value an option cannot use is
// said, status 1, where it was a usage error, 2; an option it has not got and one missing its
// argument are said with read's usage, 2; a name it cannot assign is bash's `not a valid
// identifier`, 1; and a descriptor that is not open is `invalid file descriptor: Bad file
// descriptor`, 1. Measured 2026-11-22, a script per case, with Git for Windows' bash 5.3; nemosh
// and sh keep what they said.
func TestRead_bashModeRefusesAsBash(t *testing.T) {
	const usage = "read: usage: read [-Eers] [-a array] [-d delim] [-i text] [-n nchars] [-N nchars] [-p prompt] [-t timeout] [-u fd] [name ...]\n"
	for _, test := range []struct {
		code, said string
		status     int
	}{
		{code: "read -n -1 x", said: "line 1: read: -1: invalid number\n", status: 1},
		{code: "read -N x x", said: "read: x: invalid number\n", status: 1},
		{code: "read -u -3 x", said: "read: -3: invalid file descriptor specification\n", status: 1},
		{code: "read -u 99 x", said: "read: 99: invalid file descriptor: Bad file descriptor\n", status: 1},
		{code: "read -t -0.5 x", said: "read: -0.5: invalid timeout specification\n", status: 1},
		{code: "read -Y x", said: "line 1: read: -Y: invalid option\n" + usage, status: 2},
		{code: "read -n", said: "line 1: read: -n: option requires an argument\n" + usage, status: 2},
		{code: "read 1x", said: "line 1: read: `1x': not a valid identifier\n", status: 1},
		{code: "read -a 1x", said: "read: `1x': not a valid identifier\n", status: 1},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader("in\n"), Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.RunScript(context.Background(), test.code+"\necho st=$?\n")
		want := "st=" + string(rune('0'+test.status)) + "\n"
		if stdout.String() != want || !strings.HasSuffix(stderr.String(), test.said) {
			t.Errorf("%s: stdout %q, stderr %q; want %q and %q", test.code, stdout.String(), stderr.String(), want, test.said)
		}
	}
	// As nemosh, as before.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader("in\n"), Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), "read -n -1 x\necho st=$?\nread -Y x\necho st=$?\n")
	if stdout.String() != "st=2\nst=2\n" || !strings.Contains(stderr.String(), "read: illegal option -Y\n") {
		t.Errorf("as nemosh: stdout %q, stderr %q; want what it said before", stdout.String(), stderr.String())
	}
}
