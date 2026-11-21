package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// echo -e in bash mode decodes what bash's does: an octal escape only with its 0, `\0101`, so
// `\1`, `\12` and `\123` stay as written, and `\E` as well as `\e` is escape. busybox's takes a
// bare `\NNN` too and has no `\E`, which nemosh and sh keep. Measured 2026-11-22 with Git for
// Windows' bash 5.3; Oils' builtin-echo.test.sh.
func TestEcho_bashModeEscapesAsBash(t *testing.T) {
	script := `echo -e 'a\1b\12c\123d\0101e\Ef\eg'` + "\n"
	for _, test := range []struct {
		bash bool
		want string
	}{
		{bash: true, want: "a\\1b\\12c\\123dAe\x1bf\x1bg\n"},
		{bash: false, want: "a\x01b\nc" + "SdAe\\Ef\x1bg\n"},
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
