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

func runWhich(t *testing.T, script string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
	})
	status := rt.RunScript(context.Background(), script)
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing", stderr.String())
	}
	return stdout.String(), status
}

// `which` is a builtin rather than an applet so that its answer is the shell's
// own lookup, and cannot disagree with what typing the name would run. An applet
// would have needed a second copy of PATH search and the Windows suffix order,
// and two copies drift -- which is the defect `command -v` was fixed for, when it
// said nothing about a program that plain dispatch ran perfectly well.
func TestWhich_reportsWhatTheShellWouldRun(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
		status int
	}{
		{name: "a builtin by name", script: "which cd\n", want: "cd\n"},
		{name: "an applet by name", script: "which echo\n", want: "echo\n"},
		{name: "several at once", script: "which cd echo\n", want: "cd\necho\n"},
		{name: "nothing to find is silent", script: "which zzznosuchprogram\n", want: "", status: 1},
		{name: "no operand at all", script: "which\n", want: "", status: 1},
		// One bad name among good ones still reports the good ones, and still
		// fails: a script testing the status must not be told everything is fine.
		{name: "a mixture", script: "which cd zzznosuchprogram\n", want: "cd\n", status: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, status := runWhich(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
			if status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
		})
	}
}

// `which -a` lists every match, the one that would run first, as busybox's does: it read -a as a
// name, found nothing called that and said 1, so `which -a python` failed with python on PATH.
func TestWhich_dashALists_everyMatch(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		for _, name := range []string{"prog.exe", "echo.exe"} {
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := "PATH='" + first + ";" + second + "'\n"
	spell := func(dir, name string) string { return filepath.ToSlash(filepath.Join(dir, name)) + "\n" }
	for _, test := range []struct {
		name, script, want string
		status             int
	}{
		{name: "a program twice on PATH", script: path + "which -a prog\n", want: spell(first, "prog.exe") + spell(second, "prog.exe")},
		// The applet runs, so it comes first, and then what PATH holds under its name.
		{name: "an applet and the programs it hides", script: path + "which -a echo\n", want: "echo\n" + spell(first, "echo.exe") + spell(second, "echo.exe")},
		{name: "a builtin", script: "PATH=\nwhich -a cd\n", want: "cd\n"},
		{name: "one of several names missing", script: path + "which -a prog zzznosuchprogram\n", want: spell(first, "prog.exe") + spell(second, "prog.exe"), status: 1},
		{name: "nothing at all", script: path + "which -a zzznosuchprogram\n", status: 1},
		{name: "-- ends the options", script: path + "which -- prog\n", want: spell(first, "prog.exe")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, status := runWhich(t, test.script); stdout != test.want || status != test.status {
				t.Fatalf("%q: stdout %q, status %d; want %q, %d", test.script, stdout, status, test.want, test.status)
			}
		})
	}
}

// Any other option is refused as busybox's which refuses it, rather than looked for as a command.
func TestWhich_refusesAnUnknownOption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	if status := rt.RunScript(context.Background(), "which -x ls\n"); status != 1 || stdout.Len() != 0 ||
		!strings.Contains(stderr.String(), "which: unknown option -- x") {
		t.Fatalf("which -x ls: status %d, stdout %q, stderr %q", status, stdout.String(), stderr.String())
	}
}

// And it agrees with `command -v`, which is the same lookup asked a different
// way. If these two ever differ, one of them is lying about what will run. Their
// failures are each busybox's, 1 for which and 127 for command -v.
func TestWhich_agreesWithCommandV(t *testing.T) {
	for _, name := range []string{"cd", "echo", "ls", "zzznosuchprogram"} {
		t.Run(name, func(t *testing.T) {
			fromWhich, whichStatus := runWhich(t, "which "+name+"\n")
			fromCommand, commandStatus := runWhich(t, "command -v "+name+"\n")
			if fromWhich != fromCommand || (whichStatus == 0) != (commandStatus == 0) {
				t.Fatalf("which says %q (%d) and command -v says %q (%d)",
					fromWhich, whichStatus, fromCommand, commandStatus)
			}
		})
	}
}
