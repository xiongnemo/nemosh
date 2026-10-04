package runtime_test

import (
	goruntime "runtime"
	"strings"
	"testing"
)

// command's -V and -p, which busybox has and which were taken for command names. The
// answers are busybox's -- -V in type's words and for the first name only, a name not found
// `x: not found` on stdout and 127, and -p a default PATH that finds the system's own
// programs.
func TestCommand_verboseAndDefaultPath(t *testing.T) {
	t.Run("-V", func(t *testing.T) {
		status, stdout, stderr := runSetScript(t,
			"f() { :; }\ncommand -V cd\ncommand -V ls\ncommand -V f\ncommand -v -V cd\ncommand -V nosuch\necho \"st=$?\"\n")
		want := "cd is a shell builtin\nls is a builtin applet\nf is a function\ncd is a shell builtin\nnosuch: not found\nst=127\n"
		if stdout != want || status != 0 || stderr != "" {
			t.Fatalf("got %q/%d, stderr %q; want %q/0", stdout, status, stderr, want)
		}
	})
	t.Run("-p looks where the system keeps its own", func(t *testing.T) {
		name, run, ran := "sh", "command -p sh -c 'echo ran'\n", "ran\n"
		if goruntime.GOOS == "windows" {
			name, run, ran = "where.exe", "command -p cmd.exe /c echo ran\n", "ran\r\n"
		}
		status, stdout, stderr := runSetScript(t, "PATH=/nonexistent\ncommand -pv "+name+"\n"+run)
		lines := strings.SplitAfterN(stdout, "\n", 2)
		if status != 0 || len(lines) != 2 || !strings.HasSuffix(strings.ToLower(lines[0]), "/"+name+"\n") || lines[1] != ran {
			t.Fatalf("got %q/%d, stderr %q", stdout, status, stderr)
		}
	})
	t.Run("an option it has not got", func(t *testing.T) {
		status, stdout, _ := runSetScript(t, "command -x ls 2>&1\necho \"st=$?\"\n")
		if want := "nemosh: line 1: command: illegal option -x\nst=2\n"; stdout != want || status != 0 {
			t.Fatalf("got %q/%d, want %q", stdout, status, want)
		}
	})
}
