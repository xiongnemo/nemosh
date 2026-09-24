package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// $LINENO is the line the running command starts on. Every expectation is busybox-w32's,
// measured, and bash agrees with each. It was unset, so `set -u` stopped a script that
// named it.
func TestLineno_isTheLineTheCommandStartsOn(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "one per line", script: "echo $LINENO\n\necho $LINENO\n", want: "1\n3\n"},
		{name: "two on a line", script: "echo a\necho $LINENO; echo $LINENO\n", want: "a\n2\n2\n"},
		{name: "a continued line", script: "echo a \\\n  $LINENO\necho $LINENO\n", want: "a 1\n3\n"},
		{name: "a function body", script: "f() {\n  echo $LINENO\n}\necho x\nf\n", want: "x\n2\n"},
		{name: "a one-line function", script: "\ng() { echo $LINENO; }\ng\n", want: "2\n"},
		{name: "a brace group", script: "{\n  echo $LINENO\n} || {\n  echo no\n}\n{ false; } || {\n  echo $LINENO\n}\n", want: "2\n7\n"},
		{name: "a command substitution", script: "x=$(\n  echo $LINENO\n)\necho $x\n", want: "2\n"},
		{name: "past a heredoc", script: "cat <<E\n$LINENO\nE\necho $LINENO\n", want: "1\n4\n"},
		{name: "a heredoc in a function", script: "f() {\n  cat <<E\nx\nE\n  echo $LINENO\n}\nf\n", want: "x\n5\n"},
		{name: "a for header", script: "\nfor i in $LINENO; do echo $i; done\n", want: "2\n"},
		{name: "a case selector", script: "\ncase $LINENO in\n  *) echo \"$LINENO\";;\nesac\n", want: "3\n"},
		{name: "an if condition", script: "\nif [ $LINENO = 2 ]; then echo yes; fi\n", want: "yes\n"},
		{name: "eval counts from its line", script: "\neval 'echo $LINENO\necho $LINENO'\n", want: "2\n3\n"},
		{name: "a subshell and a pipeline", script: "(echo $LINENO)\necho $LINENO | cat\n", want: "1\n2\n"},
		{name: "a background job", script: "\n{ echo $LINENO; } &\nwait\n", want: "2\n"},
		{name: "under set -u", script: "set -u\necho $LINENO\n", want: "2\n"},
		{name: "an ERR trap names the failing line", script: "trap 'echo $LINENO' ERR\n\nfalse\n", want: "3\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A sourced file counts its own lines, and the caller's resume after it.
func TestLineno_sourcedFileCountsItsOwnLines(t *testing.T) {
	sourced := filepath.Join(t.TempDir(), "lib.sh")
	if err := os.WriteFile(sourced, []byte("\necho $LINENO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "\n\n. '" + filepath.ToSlash(sourced) + "'\necho $LINENO\n"
	if stdout, _ := runScriptCapturing(script); stdout != "2\n4\n" {
		t.Fatalf("stdout = %q, want the file's line and then the caller's", stdout)
	}
}

// PS4 is expanded, so `PS4='+$LINENO: '` puts the line in front of each traced command.
// It was printed as written.
func TestLineno_reachesTheTraceThroughPS4(t *testing.T) {
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
	status := rt.RunScript(context.Background(), "PS4='+$LINENO: '\nset -x\n\necho traced\n")
	rt.CloseBatch(status)
	if !strings.Contains(stderr.String(), "+4: echo traced\n") {
		t.Fatalf("stderr = %q, want the traced command's line", stderr.String())
	}
}

// A session numbers its lines on from the ones before, as busybox's does, so a function
// typed at a prompt reports the lines it was typed on.
func TestLineno_sessionCountsOnAcrossInputs(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	for _, input := range []string{"echo $LINENO\n", "f() {\n  echo $LINENO\n}\n", "f\n"} {
		script, err := rt.ParseSessionInput(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		rt.RunInteractive(context.Background(), script)
	}
	if stdout.String() != "1\n3\n" {
		t.Fatalf("stdout = %q, want 1 and then the function's own line", stdout.String())
	}
}
