package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a command's argument that looks like an assignment has its tildes expanded as an
// assignment's are, after the `=` and after each `:`, as bash does outside its POSIX mode:
// `make PREFIX=~/x` gives make the home directory. busybox, POSIX and bash's POSIX mode leave
// the tilde as written, as nemosh and sh do. Measured 2026-11-22 with Git for Windows' bash
// 5.3; Oils' tilde.test.sh.
func TestArgumentTilde_bashModeExpandsAnAssignmentsTildes(t *testing.T) {
	const words = `x=~ a=~/x b=x:~/y --c=~ 1x=~ g=a=~ x+=~ a[1]=~ 'q'=~ x==~`
	script := "HOME=/home/bob\necho " + words + "\nset -o posix\necho " + words + "\n"
	const kept = "x=~ a=~/x b=x:~/y --c=~ 1x=~ g=a=~ x+=~ a[1]=~ q=~ x==~\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "x=/home/bob a=/home/bob/x b=x:/home/bob/y --c=~ 1x=~ g=a=~ x+=/home/bob a[1]=/home/bob q=~ x==~\n" + kept},
		{bash: false, want: kept + kept},
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
