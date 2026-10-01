package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// times and local are special builtins as busybox's ash marks them: times is one of POSIX's
// and local is ash's own. An assignment in front of one stays once it has run, and a
// redirection one cannot make ends the script, as with `:`. Both were plain: `E=x local v`
// left E unset, and `local v 2>BAD` went on. A function of either name is still called, as
// busybox calls one. Each answer was measured against busybox-w32.
func TestRuntime_localAndTimesAreSpecialAsBusyboxMarksThem(t *testing.T) {
	bad := filepath.ToSlash(filepath.Join(t.TempDir(), "no", "such", "file"))
	for _, test := range []struct {
		script, want string
		status       int
	}{
		{"f() { E=env local v=var; echo \"[$E] $v\"; }; f; echo \"[$E]\"", "[env] var\n[env]\n", 0},
		{"E=env times > /dev/null; echo \"[$E]\"", "[env]\n", 0},
		{"f() { local v 2> BAD; echo after; }; f; echo end", "", 1},
		{"times 2> BAD; echo after", "", 1},
		{"f() { command local v 2> BAD; echo \"after $?\"; }; f", "after 1\n", 0},
		{"times() { echo func; }; times", "func\n", 0},
		{"local() { echo func; }; f() { local v; }; f", "func\n", 0},
	} {
		script := strings.ReplaceAll(test.script, "BAD", bad)
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script + "\n"); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d, as busybox-w32 answers", stdout, status, test.want, test.status)
			}
		})
	}
}
