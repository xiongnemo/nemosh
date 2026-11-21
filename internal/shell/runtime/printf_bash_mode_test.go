package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bash mode's printf reads a number as bash's printf does, with strtoimax, strtoumax and
// strtold: what was read before the operand stopped being a number is the number written, and
// one too large for the conversion is the largest it holds, each said, status 1. A refusal is in
// bash's words, after the shell's prefix as a builtin's is. busybox writes zero for any of them,
// in its words, which nemosh and sh keep. Measured 2026-11-22 against Git for Windows' bash 5.3.
func TestPrintf_bashModeReadsANumberAsBash(t *testing.T) {
	for _, test := range []struct {
		args, stdout string
		said         []string
		status       int
	}{
		{args: `'[%d]' 3abc`, stdout: "[3]", said: []string{"line 1: printf: 3abc: invalid number\n"}, status: 1},
		{args: `'[%d]' ' -42 '`, stdout: "[-42]", said: []string{"printf:  -42 : invalid number\n"}, status: 1},
		{args: `'[%d]' ''`, stdout: "[0]", said: []string{"printf: : invalid number\n"}, status: 1},
		{args: `'[%d]' 0x1g`, stdout: "[1]", said: []string{"printf: 0x1g: invalid hex number\n"}, status: 1},
		{args: `'[%d]' 09`, stdout: "[0]", said: []string{"printf: 09: invalid octal number\n"}, status: 1},
		{args: `'[%d]' 99999999999999999999`, stdout: "[9223372036854775807]", said: []string{"printf: 99999999999999999999: Numerical result out of range\n"}, status: 1},
		{args: `'[%d]' -99999999999999999999x`, stdout: "[-9223372036854775808]", said: []string{"printf: -99999999999999999999x: invalid number\n"}, status: 1},
		{args: `'[%u|%x]' -1 -2`, stdout: "[18446744073709551615|fffffffffffffffe]"},
		{args: `'[%u]' 99999999999999999999`, stdout: "[18446744073709551615]", said: []string{"printf: 99999999999999999999: Numerical result out of range\n"}, status: 1},
		{args: `'[%u|%x|%u]' 18446744073709551615 18446744073709551615 9223372036854775808`, stdout: "[18446744073709551615|ffffffffffffffff|9223372036854775808]"},
		{args: `'[%5d|%-5o]' 3abc 9x`, stdout: "[    3|11   ]", said: []string{"printf: 3abc: invalid number\n", "printf: 9x: invalid number\n"}, status: 1},
		{args: `'[%f|%f|%f]' 3.5abc ' 3.5 ' nanx`, stdout: "[3.500000|3.500000|nan]", said: []string{"printf: 3.5abc: invalid number\n", "printf:  3.5 : invalid number\n", "printf: nanx: invalid number\n"}, status: 1},
		{args: `'[%f]' ''`, stdout: "[0.000000]", said: []string{"printf: : invalid number\n"}, status: 1},
		{args: `'[%d|%f]'`, stdout: "[0|0.000000]"},
		{args: `'[%k]' 1`, stdout: "[", said: []string{"line 1: printf: `k': invalid format character\n"}, status: 1},
		{args: `'a%'`, stdout: "a", said: []string{"line 1: printf: `%': missing format character\n"}, status: 1},
		{args: `'b%5' 1`, stdout: "b", said: []string{"printf: `%5': missing format character\n"}, status: 1},
		{args: ``, said: []string{"printf: usage: printf [-v var] format [arguments]\n"}, status: 2},
		{args: `-v v '[%d]' 12z`, said: []string{"printf: 12z: invalid number\n"}, status: 1},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		rt.RunScript(context.Background(), "printf "+test.args+"\necho \"|$?|$v\"\n")
		want := test.stdout + "|" + string(rune('0'+test.status)) + "|\n"
		if strings.HasPrefix(test.args, "-v") {
			want = "|1|[12]\n"
		}
		said := true
		for _, message := range test.said {
			said = said && strings.Contains(stderr.String(), message)
		}
		if stdout.String() != want || !said || len(test.said) == 0 && stderr.Len() != 0 {
			t.Errorf("printf %s: stdout %q, stderr %q; want %q and %q", test.args, stdout.String(), stderr.String(), want, test.said)
		}
	}
	// As nemosh, busybox's.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), "printf '[%d|%f]' 3abc 3.5x\necho \"|$?\"\n")
	if stdout.String() != "[0|0.000000]|1\n" || !strings.Contains(stderr.String(), "printf: invalid number '3abc'\n") {
		t.Errorf("as nemosh: stdout %q, stderr %q; want busybox's zeros and words", stdout.String(), stderr.String())
	}
}
