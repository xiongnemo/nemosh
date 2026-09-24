package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `>(cmd)` is a pipe the consumer writes into while the command reads it; it was refused by
// name. runScriptCapturing closes the shell, which waits for the command, so its output is
// all there when the test reads it. Each answer is bash 5.3's, measured.
func TestOutputSubstitution_feedsTheCommandWhileItRuns(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "a redirection into it", script: "echo hi > >(cat)\n", want: "hi\n"},
		{name: "tee in a pipeline, its own output discarded", script: "echo hello | tee >(tr a-z A-Z) >/dev/null\n", want: "HELLO\n"},
		{name: "a count of what went in", script: "printf 'a\\nb\\n' > >(wc -l)\n", want: "2\n"},
		{name: "a path never opened lets the command end", script: "p=$(echo >(cat))\n[ -n \"$p\" ] && echo path\necho after\n", want: "path\nafter\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); strings.TrimLeft(stdout, " ") != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// `exec > >(tee log)` is the logging idiom: everything after it goes to the screen and the
// log, and the log is complete once the script is done.
func TestOutputSubstitution_execRedirectLogsEverything(t *testing.T) {
	log := filepath.ToSlash(filepath.Join(t.TempDir(), "log"))
	stdout, _ := runScriptCapturing("exec > >(tee '" + log + "')\necho one\necho two\n")
	if stdout != "one\ntwo\n" {
		t.Fatalf("stdout = %q, want both lines", stdout)
	}
	if content, err := os.ReadFile(log); err != nil || string(content) != "one\ntwo\n" {
		t.Fatalf("log = %q (err %v), want both lines", content, err)
	}
}
