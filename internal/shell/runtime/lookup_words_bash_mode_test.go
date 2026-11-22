package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode a command that cannot be run is said in bash's words, with no hint after them:
// `x: command not found` for a name, `./x: No such file or directory` for a path, `./d: Is a
// directory`, and the status 127 or 126 as before. nemosh and sh say `not found`, `Permission
// denied` for a directory too, as busybox does, and the hint that follows, which is nemosh's own.
// Measured 2026-11-22 with Git for Windows' bash 5.3.
func TestLookupWords_bashModeAreBashs(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	const code = "nosuchcmd-zz; echo $?\n./missing-zz; echo $?\n./d; echo $?\n"
	for _, test := range []struct {
		bash           bool
		stdout, stderr string
	}{
		{true, "127\n127\n126\n", "nemosh: line 2: nosuchcmd-zz: command not found\nnemosh: line 3: ./missing-zz: No such file or directory\nnemosh: line 4: ./d: Is a directory\n"},
		{false, "127\n127\n126\n", "nemosh: line 2: nosuchcmd-zz: not found\nhint: no directory on PATH holds nosuchcmd-zz; `command -v nosuchcmd-zz` answers the same question\n" +
			"nemosh: line 3: ./missing-zz: not found\nhint: ./missing-zz was read as a path, not a PATH lookup; check the spelling and that the file exists\n" +
			"nemosh: line 4: ./d: Permission denied\nhint: ./d is a directory, which is not executable\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		if test.bash {
			rt.EnterBashMode()
		}
		rt.RunScript(context.Background(), "cd '"+filepath.ToSlash(dir)+"'\n"+code)
		if stdout.String() != test.stdout || stderr.String() != test.stderr {
			t.Errorf("bash mode %v:\nstdout %q\nstderr %q\nwant   %q\nand    %q", test.bash, stdout.String(), stderr.String(), test.stdout, test.stderr)
		}
	}
}
