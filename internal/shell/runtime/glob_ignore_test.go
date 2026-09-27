package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// GLOBIGNORE leaves out of a glob's pathnames the ones that match its colon-separated
// patterns, matched against the whole path; with it set a glob matches names beginning with a
// dot, and a glob it empties stays as written, or goes under nullglob. It was not read at all.
// The answers are bash 5.3's; busybox has no GLOBIGNORE.
func TestRuntime_globIgnoreLeavesPathsOut(t *testing.T) {
	tests := []struct {
		setup, script, want string
	}{
		{"touch one.md one.txt; mkdir foo; touch foo/two.md foo/two.txt", "GLOBIGNORE=*.txt; echo *.* foo/*.*", "one.md foo/two.md foo/two.txt\n"},
		{"touch hello.c hello.h hello.o hello", "GLOBIGNORE=*.o:*.h; echo hello*", "hello hello.c\n"},
		{"mkdir d1 d2; touch d1/a.txt d1/ignore.txt d2/a.txt d2/ignore.txt", "GLOBIGNORE='*/ignore*'; echo */*", "d1/a.txt d2/a.txt\n"},
		{"touch 1.txt 2.log", "GLOBIGNORE=*; echo *; shopt -s nullglob; echo \"[$(echo *)]\"", "*\n[]\n"},
		{"touch .env _t.py x.log", "GLOBIGNORE='[[:alnum:]]*'; echo *.*", ".env _t.py\n"},
		{"touch 'escape-10.txt' 'escape*.txt'", "GLOBIGNORE='escape\\*.txt'; echo *.*", "escape-10.txt\n"},
		{"touch reset.txt", "GLOBIGNORE=*.txt; echo *.*; GLOBIGNORE=; echo *.*", "*.*\nreset.txt\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			script := fmt.Sprintf("cd '%s'; %s\n%s\n", filepath.ToSlash(t.TempDir()), test.setup, test.script)
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
