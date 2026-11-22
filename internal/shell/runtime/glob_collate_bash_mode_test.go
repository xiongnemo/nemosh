package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a pattern's matches are sorted as the locale collates them, LC_ALL over
// LC_COLLATE over LANG, each a shell variable whether exported or not; with none of them, or C,
// POSIX or C.anything, by byte. Git bash collates through Windows, so en_US puts `_x` first and
// `hello.py` before `hello_preamble.sh`, which glibc's en_US does not. busybox sorts by byte
// whatever the locale, and so do nemosh and sh. Measured 2026-11-22 with Git for Windows' bash
// 5.3; Oils' glob.test.sh.
func TestGlobCollate_bashModeFollowsTheLocale(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"hello", "hello.py", "hello_preamble.sh", "hello-test.sh", "_x", "a", "a b", "B"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const bytewise = "B _x a a b hello hello-test.sh hello.py hello_preamble.sh\n"
	const collated = "_x a a b B hello hello.py hello_preamble.sh hello-test.sh\n"
	for _, test := range []struct {
		code, stdout string
		bash         bool
	}{
		{code: "echo *", stdout: bytewise, bash: true},
		{code: "LC_COLLATE=en_US.UTF-8; echo *", stdout: collated, bash: true},
		{code: "LANG=en_US; echo *", stdout: collated, bash: true},
		{code: "LANG=de_DE.UTF-8 LC_COLLATE=C.UTF-8; echo *", stdout: bytewise, bash: true},
		{code: "LC_COLLATE=en_US.UTF-8 LC_ALL=POSIX; echo *", stdout: bytewise, bash: true},
		{code: "LC_ALL=en_US.UTF-8 LANG=C; echo *", stdout: collated, bash: true},
		{code: "LC_COLLATE=en_US.UTF-8; echo *", stdout: bytewise},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		status := rt.RunScript(context.Background(), "unset LC_ALL LC_COLLATE LANG; cd '"+filepath.ToSlash(dir)+"' || exit 9\n"+test.code+"\n")
		if status != 0 || stdout.String() != test.stdout {
			t.Errorf("bash mode %v, %q: status %d, stdout %q, stderr %q; want %q", test.bash, test.code, status, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
