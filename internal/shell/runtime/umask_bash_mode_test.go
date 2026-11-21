package runtime

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bash mode's umask is bash 5.3's where it parts from busybox's: it starts at 0022; a symbolic
// mode takes X, s, t and a class to copy, as POSIX has them, with a clause that names no class
// meaning all three, unfiltered; and a mode it refuses is said in bash's words, status 1.
// Measured 2026-11-22, a script per case, against Git for Windows' bash 5.3; nemosh and sh keep
// busybox's, which the last cases hold.
func TestUmask_bashModeReadsAModeAsBash(t *testing.T) {
	const usage = "umask: usage: umask [-p] [-S] [mode]\n"
	for _, test := range []struct {
		start, operand, printed, after, said string
		status                               int
	}{
		{operand: "", printed: "0022\n"},
		{operand: "-S", printed: "u=rwx,g=rx,o=rx\n"},
		{start: "0124", operand: "a=X", after: "0666"},
		{start: "0246", operand: "a=X", after: "0666"},
		{start: "0111", operand: "a+X", after: "0111"},
		{start: "0111", operand: "u+x,g+X", after: "0011"},
		{start: "0124", operand: "a-X", after: "0135"},
		{start: "0124", operand: "a+s", after: "0124"},
		{start: "0124", operand: "a=s", after: "0777"},
		{start: "0124", operand: "g=t", after: "0174"},
		{start: "0124", operand: "a=u", after: "0111"},
		{start: "0365", operand: "a=g", after: "0666"},
		{start: "0124", operand: "a=,a=u", after: "0111"},
		{start: "0124", operand: "o=ug", after: "0122"},
		{start: "0124", operand: "u=gw", after: "0024"},
		{start: "0124", operand: "a=u+o", after: "0000"},
		{start: "0124", operand: "+g", after: "0020"},
		{start: "0124", operand: "=rx", after: "0222"},
		{start: "0124", operand: "+r", after: "0120"},
		{start: "0124", operand: "=+=", after: "0777"},
		{start: "0124", operand: "=+rwx+rx", after: "0000"},
		{start: "0124", operand: "7777", after: "0777"},
		{start: "0124", operand: "-S 0777", printed: "u=,g=,o=\n", after: "0777"},
		{start: "0124", operand: "-p 0777", after: "0777"},
		{start: "0124", operand: "u=r g=w", after: "0324"},
		{start: "0124", operand: "''", after: "0124", said: "umask: `\x00': invalid symbolic mode operator\n", status: 1},
		{start: "0124", operand: "u=, g+, o-", after: "0124", said: "umask: `\x00': invalid symbolic mode operator\n", status: 1},
		{start: "0124", operand: "'u=r g=w'", after: "0124", said: "umask: ` ': invalid symbolic mode character\n", status: 1},
		{start: "0124", operand: "u+r,,u-r", after: "0124", said: "umask: `,': invalid symbolic mode operator\n", status: 1},
		{start: "0124", operand: "b=rwx", after: "0124", said: "umask: `b': invalid symbolic mode operator\n", status: 1},
		{start: "0124", operand: "u+q", after: "0124", said: "umask: `q': invalid symbolic mode character\n", status: 1},
		{start: "0124", operand: "089", after: "0124", said: "umask: 089: octal number out of range\n", status: 1},
		{start: "0124", operand: "1234567", after: "0124", said: "umask: 1234567: octal number out of range\n", status: 1},
		{start: "0124", operand: "-rwx", after: "0124", said: "umask: -r: invalid option\n" + usage, status: 2},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		script, want := "umask "+test.operand+"\n", test.printed
		if test.start != "" {
			script = "umask " + test.start + "\n" + script + "echo status=$?\numask\n"
			want += fmt.Sprintf("status=%d\n%s\n", test.status, test.after)
		}
		if rt.RunScript(context.Background(), script); stdout.String() != want || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
			t.Errorf("umask %s under %s: stdout %q, stderr %q; want %q and %q", test.operand, test.start, stdout.String(), stderr.String(), want, test.said)
		}
	}
	// As nemosh, busybox's.
	for script, want := range map[string]string{
		"umask\n": "0002\n", "umask 0124\numask =rx\numask\n": "0326\n", "umask 0124\numask a=u\numask\n": "0777\n",
		"umask 0124\numask ''\necho $?\n": "0\n",
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if rt.RunScript(context.Background(), script); stdout.String() != want {
			t.Errorf("as nemosh %q: stdout %q, stderr %q; want busybox's %q", script, stdout.String(), stderr.String(), want)
		}
	}
}
