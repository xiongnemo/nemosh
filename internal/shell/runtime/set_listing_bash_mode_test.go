package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// set with no arguments lists the variables as bash's does in bash mode: a value as it is when
// it reads back as itself, single-quoted, a quote written as an escaped one between quoted parts, when it holds a character the
// shell treats specially, $'...' when it holds a control character, and nothing for the empty
// one -- and then the functions, as declare -f prints them, but under set -o posix. busybox's
// showvars single-quotes every value its own way, `a='spam'`, and lists no function, which
// nemosh and sh keep. Measured 2026-11-22 with Git for Windows' bash 5.3; Oils' serialize,
// sh-options, builtin-misc and known-differences cases.
func TestSetVariableListing_bashModeIsBashs(t *testing.T) {
	script := "a=spam; b='two words'; c=$'a\\nb\\001c\\'d'; d=''; e=\"it's\"; f='$x'\nf1() { echo hi; }\n"
	for _, test := range []struct {
		bash  bool
		code  string
		lines []string
		not   []string
	}{
		{bash: true, code: "set", lines: []string{"a=spam\n", "b='two words'\n", "c=$'a\\nb\\001c\\'d'\n", "d=\n", "e='it'\\''s'\n", "f='$x'\n", "f1 () \n{ \n    echo hi\n}\n"}},
		{bash: true, code: "set -o posix; set", lines: []string{"a=spam\n"}, not: []string{"f1 ()"}},
		{bash: false, code: "set", lines: []string{"a='spam'\n", "d=''\n", "e='it'\"'\"'s'\n"}, not: []string{"f1 ()"}},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script+test.code+"\n")
		listing := "\n" + stdout.String()
		for _, line := range test.lines {
			if !strings.Contains(listing, "\n"+line) {
				t.Errorf("bash mode %v, %s: listing lacks %q:\n%s", test.bash, test.code, line, stdout.String())
			}
		}
		for _, absent := range test.not {
			if strings.Contains(listing, "\n"+absent) {
				t.Errorf("bash mode %v, %s: listing has %q", test.bash, test.code, absent)
			}
		}
	}
}
