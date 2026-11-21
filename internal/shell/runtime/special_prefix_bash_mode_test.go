package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An assignment in front of a special builtin is the builtin's alone in bash mode, as bash has it
// outside its POSIX mode: seen while it runs -- by eval's text and a sourced file -- and gone
// after it, `x=1 :`, `x=1 export y=2` and `x=1 local v` alike. Under set -o posix it stays, as
// POSIX and busybox have it, and as nemosh and sh keep it. Measured 2026-11-22, a script per
// case, with Git for Windows' bash 5.3; Oils' builtin-special.test.sh and command-parsing.test.sh.
func TestSpecialPrefix_bashModeIsTheBuiltinsAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dot.sh"), []byte("echo in-dot=$x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "cd \"" + filepath.ToSlash(dir) + "\"\n" +
		"x=1 :; echo colon=$x; unset x\n" +
		"x=1 eval 'echo in-eval=$x'; echo eval=$x; unset x\n" +
		"x=1 export y=2; echo export=$x y=$y; unset x\n" +
		"x=1 . ./dot.sh; echo dot=$x; unset x\n" +
		"x=1 set -- a b; echo set=$x $#; unset x\n" +
		"f() { x=1 local v=2; echo local-in=$x v=$v; }; f; echo local=$x; unset x\n" +
		"x=1 eval 'z=3'; echo z=$z\n" +
		"set -o posix\n" +
		"x=1 :; echo posix-colon=$x; unset x\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "colon=\nin-eval=1\neval=\nexport= y=2\nin-dot=1\ndot=\nset= 2\nlocal-in= v=2\nlocal=\nz=3\nposix-colon=1\n"},
		{bash: false, want: "colon=1\nin-eval=1\neval=1\nexport=1 y=2\nin-dot=1\ndot=1\nset=1 2\nlocal-in=1 v=2\nlocal=1\nz=3\nposix-colon=1\n"},
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
