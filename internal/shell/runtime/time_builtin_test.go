package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// time runs its command and then reports real, user and sys on stderr, in busybox-w32's
// shape (miscutils/time.c). The shell had no time: `time cmd` found time.exe on PATH or was
// not found. A function can be timed too, and a failing command is reported above the times.
func TestTime_runsTheCommandAndReportsItsTimes(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	minutes := `\d+m \d+\.\d\ds`
	for index, test := range []struct {
		script, stdout, stderr string
		status                 int
	}{
		{"time echo hi\n", "hi\n", `^real\t` + minutes + `\nuser\t` + minutes + `\nsys\t` + minutes + `\n$`, 0},
		{"time false\n", "", `^Command exited with non-zero status 1\nreal\t`, 1},
		{"time -p true\n", "", `^real \d+\.\d\d\nuser \d+\.\d\d\nsys \d+\.\d\d\n$`, 0},
		{"time -f '%C:%x %% \\t|' false x\n", "", "^Command exited with non-zero status 1\nfalse x:1 % \t\\|\n$", 1},
		{"f() { echo in-f; }\ntime -f '' f\n", "in-f\n", `^\n$`, 0},
		{"export TIME='[%x]'\ntime true\n", "", `^\[0\]\n$`, 0},
		{"time -f '%q' true\n", "", `^\?q\n$`, 0},
		{"time\n", "", `^time: expected a command to run\n$`, 1},
		{"time -o '" + dir + "/t.txt' -f 'took' true\n", "", `^$`, 0},
	} {
		var stdout, stderr bytes.Buffer
		rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
		status := rt.RunScript(context.Background(), test.script)
		rt.CloseBatch(status)
		if status != test.status || stdout.String() != test.stdout || !regexp.MustCompile(test.stderr).MatchString(stderr.String()) {
			t.Errorf("%d: %q: got %q/%d, stderr %q; want %q/%d and stderr like %q",
				index, test.script, stdout.String(), status, stderr.String(), test.stdout, test.status, test.stderr)
		}
	}
	if report, err := os.ReadFile(filepath.Join(dir, "t.txt")); err != nil || string(report) != "took\n" {
		t.Errorf("time -o wrote %q, %v; want %q", report, err, "took\n")
	}
}
