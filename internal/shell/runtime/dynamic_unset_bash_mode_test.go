package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode unset of a variable a caller made local takes that local away, as bash's does with
// localvar_unset off, its default: the value the caller's local hid is the variable's again, and
// an assignment after is to it. Unset in the function that made it local leaves it unset there,
// as before. `shopt -s localvar_unset` asks for the second everywhere, which is busybox's, and
// which nemosh and sh keep, starting with the option on. Measured 2026-11-22, a script per case,
// with Git for Windows' bash 5.3 and busybox-w32; Oils' builtin-vars.test.sh, `Unset and scope`.
func TestDynamicUnset_bashModeTakesTheCallersLocalAway(t *testing.T) {
	script := `f() { local x=f; g; echo "f after g: ${x-unset}"; x=again; }
g() { unset x; echo "g: ${x-unset}"; }
x=global; f; echo "top: ${x-unset}"
h() { local y=h; unset y; echo "h: ${y-unset}"; y=new; }; y=global; h; echo "top y: $y"
a() { local z=a; b; echo "a: ${z-unset}"; }
b() { local z=b; c; echo "b: ${z-unset}"; }
c() { unset z; echo "c1: ${z-unset}"; unset z; echo "c2: ${z-unset}"; }
z=global; a; echo "top z: ${z-unset}"
shopt -s localvar_unset
x=global; f; echo "top: ${x-unset}"
shopt localvar_unset
`
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "g: global\nf after g: global\ntop: again\nh: unset\ntop y: global\n" +
			"c1: a\nc2: global\nb: global\na: global\ntop z: global\n" +
			"g: unset\nf after g: unset\ntop: global\nlocalvar_unset      \ton\n"},
		{bash: false, want: "g: unset\nf after g: unset\ntop: global\nh: unset\ntop y: global\n" +
			"c1: unset\nc2: unset\nb: unset\na: a\ntop z: global\n" +
			"g: unset\nf after g: unset\ntop: global\nlocalvar_unset      \ton\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), script)
		if stdout.String() != test.want {
			t.Errorf("bash mode %v: stdout %q, want %q; stderr %q", test.bash, stdout.String(), test.want, stderr.String())
		}
	}
	// bash mode starts with the option off, as bash does; nemosh and sh with it on.
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &bytes.Buffer{}})
	rt.EnterBashMode()
	rt.RunScript(context.Background(), "shopt localvar_unset\n")
	if stdout.String() != "localvar_unset      \toff\n" {
		t.Errorf("bash mode's localvar_unset: %q, want off", stdout.String())
	}
}
