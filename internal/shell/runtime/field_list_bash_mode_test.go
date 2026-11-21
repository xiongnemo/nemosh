package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Unquoted, bash's $@ and $* are the parameters joined by IFS's first character and then split,
// so an empty parameter between two others is an empty field where IFS has a character that is
// not blank, and a separator at a parameter's edge meets the joining one: with IFS=x, `ax b` is
// a, an empty field and b. busybox splits each parameter on its own, which nemosh and sh keep.
// An array is bash's alone, and its unquoted @ and * forms are bash's under every name. Measured
// 2026-11-22, a script per case, with Git for Windows' bash 5.3.
func TestUnquotedList_bashModeJoinsByIFSThenSplits(t *testing.T) {
	for _, test := range []struct {
		code, want string
		bash       bool
	}{
		{code: `IFS=x; set -- ax b; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><><b>\n", bash: true},
		{code: `IFS=x; set -- a xb; for f in $*; do printf '<%s>' "$f"; done; echo`, want: "<a><><b>\n", bash: true},
		{code: `IFS=x; set -- '' a; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<><a>\n", bash: true},
		{code: `IFS=x; set -- '' ''; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<>\n", bash: true},
		{code: `IFS=x; set -- ''; for f in =$@=; do printf '<%s>' "$f"; done; echo`, want: "<==>\n", bash: true},
		{code: `IFS=zx; set -- one '' two; for f in $*; do printf '<%s>' "$f"; done; echo`, want: "<one><><two>\n", bash: true},
		{code: `IFS='x '; set -- a '' b; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><><b>\n", bash: true},
		{code: `IFS=' x'; set -- a xb; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><b>\n", bash: true},
		{code: `IFS=; set -- a '' b; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><b>\n", bash: true},
		{code: `set -- a '' b; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><b>\n", bash: true},
		{code: `IFS=x; set -- "" "" "" "" ""; set -- $*; echo $#; set -- $*; echo $#`, want: "4\n3\n", bash: true},
		// An array, under either name.
		{code: `IFS=x; a=(ax b); for f in ${a[@]}; do printf '<%s>' "$f"; done; echo`, want: "<a><><b>\n"},
		{code: `IFS=x; a=('' a ''); for f in =${a[*]}=; do printf '<%s>' "$f"; done; echo`, want: "<=><a><=>\n"},
		// As nemosh, busybox's.
		{code: `IFS=x; set -- ax b; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a><b>\n"},
		{code: `IFS=x; set -- '' a; for f in $@; do printf '<%s>' "$f"; done; echo`, want: "<a>\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), test.code+"\n")
		if stdout.String() != test.want || stderr.Len() != 0 {
			t.Errorf("bash mode %v, %s: stdout %q, stderr %q; want %q", test.bash, test.code, stdout.String(), stderr.String(), test.want)
		}
	}
}
