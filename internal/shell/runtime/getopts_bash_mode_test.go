package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// getopts in bash mode is bash's sh_getopt, where busybox's parts from it: OPTIND names the word
// being read until its last letter is, `-ab` reading a with OPTIND still 1; a flag leaves OPTARG
// unset; `set --` and shift leave the parse where it was, and a parse past the words ends with
// OPTIND one past them rather than starting again; a name that is no variable's is status 1,
// OPTIND and OPTARG set; and the diagnostics are bash's, `$0: illegal option -- z`. Each answer
// is Oils' builtin-getopts.test.sh's for bash or Git for Windows' bash 5.3's, measured
// 2026-11-21; nemosh and sh keep busybox's.
func TestGetopts_bashModeIsBashs(t *testing.T) {
	for _, test := range []struct {
		name, script, stdout, said string
	}{
		{name: "two flags: -ab", script: "getopts ab opt -ab; echo OPTIND=$OPTIND opt=$opt\ngetopts ab opt -ab; echo OPTIND=$OPTIND opt=$opt\n",
			stdout: "OPTIND=1 opt=a\nOPTIND=2 opt=b\n"},
		{name: "more smooshing", script: "for i in 1 2 3; do getopts abc: opt -abc10; echo OPTIND=$OPTIND opt=$opt OPTARG=$OPTARG; done\n",
			stdout: "OPTIND=1 opt=a OPTARG=\nOPTIND=1 opt=b OPTARG=\nOPTIND=2 opt=c OPTARG=10\n"},
		{name: "an argument in the next word", script: "for i in 1 2 3; do getopts ab:c: opt -ab hi -c hello; echo OPTIND=$OPTIND opt=$opt OPTARG=$OPTARG; done\n",
			stdout: "OPTIND=1 opt=a OPTARG=\nOPTIND=3 opt=b OPTARG=hi\nOPTIND=5 opt=c OPTARG=hello\n"},
		{name: "a flag leaves OPTARG unset", script: "OPTARG=x; getopts ab name -a; echo name=$name OPTARG=${OPTARG-unset}\n",
			stdout: "name=a OPTARG=unset\n"},
		{name: "set -- leaves the parse where it was",
			script: "set -- -a\nwhile getopts ab: opt; do echo .; done; echo OPTIND=$OPTIND\nset -- -c -d -e foo\nwhile getopts cde: opt; do echo -; done; echo OPTIND=$OPTIND\nset -- -f\nwhile getopts f: opt; do echo _; done; echo OPTIND=$OPTIND\n",
			stdout: ".\nOPTIND=2\n-\n-\nOPTIND=5\nOPTIND=2\n"},
		{name: "a name that is no variable's", script: "set -- -c foo -h\ngetopts hc: opt-\necho status=$? opt=$opt OPTARG=$OPTARG OPTIND=$OPTIND\n",
			stdout: "status=1 opt= OPTARG=foo OPTIND=3\n", said: "getopts: `opt-': not a valid identifier\n"},
		{name: "an option it has not got", script: "getopts ab: opt -z; echo st=$? opt=$opt OPTARG=${OPTARG-unset}\n",
			stdout: "st=0 opt=? OPTARG=unset\n", said: "script: illegal option -- z\n"},
		{name: "an argument missing", script: "getopts ab: opt -b; echo st=$? opt=$opt OPTARG=${OPTARG-unset}\n",
			stdout: "st=0 opt=? OPTARG=unset\n", said: "script: option requires an argument -- b\n"},
		{name: "silent", script: "getopts :ab: opt -z; echo opt=$opt OPTARG=$OPTARG\nOPTIND=1; getopts :ab: opt -b; echo opt=$opt OPTARG=$OPTARG\n",
			stdout: "opt=? OPTARG=z\nopt=: OPTARG=b\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			rt.EnterBashMode()
			rt.SetArguments("script", nil)
			rt.RunScript(context.Background(), test.script)
			if stdout.String() != test.stdout || !strings.HasSuffix(stderr.String(), test.said) || test.said == "" && stderr.Len() != 0 {
				t.Fatalf("stdout %q, stderr %q; want %q and %q", stdout.String(), stderr.String(), test.stdout, test.said)
			}
		})
	}
	// As nemosh, busybox's: OPTIND past the word at once.
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.RunScript(context.Background(), "getopts ab opt -ab; echo OPTIND=$OPTIND opt=$opt\n")
	if stdout.String() != "OPTIND=2 opt=a\n" {
		t.Errorf("as nemosh: %q, want busybox's OPTIND=2", stdout.String())
	}
}
