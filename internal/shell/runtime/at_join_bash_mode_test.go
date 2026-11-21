package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Where nothing splits it -- an assignment, a case word, [[ ]], a here-string, an operator's
// word -- bash joins $@ with a space whatever IFS is, and $* with IFS's first character. busybox
// joins both with IFS's first character, which nemosh and sh keep. Measured 2026-11-22, a script
// per case, with Git for Windows' bash 5.3 and busybox-w32.
func TestAtJoin_bashModeJoinsWithASpace(t *testing.T) {
	const setup = "IFS=:; set -- x 'y z'\n"
	for _, test := range []struct {
		code, want string
		bash       bool
	}{
		{code: `s=$@; echo "$s"`, want: "x y z\n", bash: true},
		{code: `s="$@"; echo "$s"`, want: "x y z\n", bash: true},
		{code: `s="${@}"; echo "$s"`, want: "x y z\n", bash: true},
		{code: `s="a$@b"; echo "$s"`, want: "ax y zb\n", bash: true},
		{code: `s=$*; echo "$s"`, want: "x:y z\n", bash: true},
		{code: `s="$*"; echo "$s"`, want: "x:y z\n", bash: true},
		{code: `case $@ in 'x y z') echo space;; *) echo other;; esac`, want: "space\n", bash: true},
		{code: `case "$@" in 'x y z') echo space;; *) echo other;; esac`, want: "space\n", bash: true},
		{code: `[[ "$@" == 'x y z' ]] && echo space`, want: "space\n", bash: true},
		{code: `[[ $* == 'x:y z' ]] && echo colon`, want: "colon\n", bash: true},
		{code: `cat <<< "$@"`, want: "x y z\n", bash: true},
		{code: `cat <<< $*`, want: "x:y z\n", bash: true},
		{code: `s=${q:-"$@"}; echo "$s"`, want: "x y z\n", bash: true},
		{code: `s=${@:-q}; echo "$s"`, want: "x y z\n", bash: true},
		{code: `export e=$@; declare d=$@; echo "$e|$d"`, want: "x y z|x y z\n", bash: true},
		{code: `IFS=; s=$@; t=$*; echo "$s|$t"`, want: "x y z|xy z\n", bash: true},
		// As nemosh, busybox's.
		{code: `s=$@; t="$@"; echo "$s|$t"`, want: "x:y z|x:y z\n"},
		{code: `case $@ in 'x:y z') echo colon;; *) echo other;; esac`, want: "colon\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), setup+test.code+"\n")
		if stdout.String() != test.want || stderr.Len() != 0 {
			t.Errorf("bash mode %v, %s: stdout %q, stderr %q; want %q", test.bash, test.code, stdout.String(), stderr.String(), test.want)
		}
	}
}
