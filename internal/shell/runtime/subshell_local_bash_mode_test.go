package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a subshell in a function is still in that function, as bash's is: `local` works
// there, and an unset of the caller's local takes it away, with the caller's own locals copied
// into the subshell and left alone outside it. busybox's subshell is in no function, and its
// local refuses, `local: not in a function`, which nemosh and sh keep. Measured 2026-11-22 with
// Git for Windows' bash 5.3 and busybox-w32.
func TestSubshellLocal_bashModeIsInTheFunction(t *testing.T) {
	script := `f() { local l=1 a=(p q); ( local x=2; echo "sub=$x l=$l a=${a[1]}"; l=changed; a[1]=r ); echo "st=$? l=$l a=${a[1]}"; }
f; echo "after=${l-unset} ${x-unset}"
unlocal() { unset -v "$1"; }
g() { local v=g; ( unlocal v; echo "sub v=$v" ); echo "g v=$v"; }
v=global; g
`
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "sub=2 l=1 a=q\nst=0 l=1 a=q\nafter=unset unset\nsub v=global\ng v=g\n"},
		{bash: false, want: "st=2 l=1 a=q\nafter=unset unset\nsub v=\ng v=g\n"},
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
}
