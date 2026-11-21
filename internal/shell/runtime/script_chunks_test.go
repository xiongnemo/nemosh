package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A script that does not parse whole is read a command at a time in bash mode, as bash reads
// every script: what comes before the bad text runs, and the syntax error ends the script there,
// status 2, its EXIT trap run; text after an exit is never read. As nemosh, the script is refused
// before its first line runs, as it was. Each answer is Git for Windows' bash 5.3's, measured
// 2026-11-21, or Oils' (builtin-trap.test.sh, parse-errors.test.sh) for bash.
func TestScript_bashModeReadsACommandAtATimeWhenItDoesNotParseWhole(t *testing.T) {
	for _, test := range []struct {
		name, script, bash string
		status             int
	}{
		{name: "a syntax error later", script: "echo before\nif then\necho after\n", bash: "before\n", status: 2},
		{name: "text after exit", script: "echo before\nexit 0\n((((\n)) ]] fi\n", bash: "before\n", status: 0},
		{name: "unterminated at the end", script: "echo a\nif true; then\n  echo b\n", bash: "a\n", status: 2},
		{name: "the EXIT trap runs", script: "trap 'echo FAILED' EXIT\nfor\n", bash: "FAILED\n", status: 2},
		{name: "the EXIT trap's exit decides", script: "trap 'echo FAILED; exit 0' EXIT\nfor\n", bash: "FAILED\n", status: 0},
		{name: "$? goes on from command to command", script: "false\necho $?\nif then\n", bash: "1\n", status: 2},
		{name: "$LINENO counts the lines", script: "echo $LINENO\n\necho $LINENO\nif then\n", bash: "1\n3\n", status: 2},
		{name: "a here-document and a loop before", script: "cat <<EOF\nhi\nEOF\nfor i in 1 2\ndo\n  echo $i\ndone\nif then\n", bash: "hi\n1\n2\n", status: 2},
		{name: "errexit before the rest is read", script: "set -o errexit\n{ls;\n}\n", bash: "", status: 127},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, bash := range []bool{false, true} {
				var stdout, stderr bytes.Buffer
				rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
				want, status := "", 2
				if bash {
					rt.EnterBashMode()
					want, status = test.bash, test.status
				}
				got := rt.RunScript(context.Background(), test.script)
				if got != status || stdout.String() != want {
					t.Errorf("bash mode %v: status %d, stdout %q, stderr %q; want %d and %q", bash, got, stdout.String(), stderr.String(), status, want)
				}
			}
		})
	}
	// eval's text and a sourced file are read so too: `eval $'echo a\nif then'` says a, where
	// one line, `echo a; if then`, is one command bash cannot parse and runs nothing of. The
	// script goes on after either, status 2, as M5 has it. As nemosh, nothing runs and the
	// script ends there.
	source := filepath.Join(t.TempDir(), "late.sh")
	if err := os.WriteFile(source, []byte("echo a\nif then\necho c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ script, bash string }{
		{script: "eval $'echo a\\nif then'; echo after=$?\n", bash: "a\nafter=2\n"},
		{script: "eval 'echo a; if then'; echo after=$?\n", bash: "after=2\n"},
		{script: ". '" + filepath.ToSlash(source) + "'; echo after=$?\n", bash: "a\nafter=2\n"},
	} {
		for _, bash := range []bool{false, true} {
			var stdout bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
			want := ""
			if bash {
				rt.EnterBashMode()
				want = test.bash
			}
			rt.RunScript(context.Background(), test.script)
			if stdout.String() != want {
				t.Errorf("%q, bash mode %v: stdout %q, want %q", test.script, bash, stdout.String(), want)
			}
		}
	}
	// A long loop before a late error is read once, not again for every line of it.
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.EnterBashMode()
	script := "for i in 1; do\n" + strings.Repeat("  :\n", 3000) + "done\necho mid\nif then\n"
	if got := rt.RunScript(context.Background(), script); got != 2 || stdout.String() != "mid\n" {
		t.Errorf("a long loop: status %d, stdout %q", got, stdout.String())
	}
}
