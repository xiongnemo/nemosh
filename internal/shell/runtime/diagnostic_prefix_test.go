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

// runNamed runs script as $0 name, and as the file of that name when file is set, and answers
// what it said on standard error without the hint lines, whose words are their own.
func runNamed(t *testing.T, name string, file bool, script string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	rt.SetArguments(name, nil)
	if file {
		rt.SetScriptFile(name)
	}
	rt.RunScript(context.Background(), script)
	var kept []string
	for _, line := range strings.SplitAfter(stderr.String(), "\n") {
		if !strings.HasPrefix(line, "hint: ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}

// A message begins as bash's error_prolog begins one: the file the failing command is in -- the
// script, a sourced file, the file a function was defined in -- and the command's line there.
// `$(< missing)` names the line it is on, where it named line 1. Each line is bash 5.3's but
// for the words after the prefix. They all began `nemosh: `, which said neither.
func TestDiagnosticPrefix_namesTheFileAndTheLine(t *testing.T) {
	dir := t.TempDir()
	library := "f() {\n  nosuch-in-f-zz\n}\nnosuch-lib-zz\n"
	if err := os.WriteFile(filepath.Join(dir, "lib.sh"), []byte(library), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "cd '" + filepath.ToSlash(dir) + "'\ncd nosuch-dir-zz\n. ./lib.sh\nf\nx=$(< missing)\n"
	want := "build.sh: line 2: cd: cannot cd to nosuch-dir-zz: No such file or directory\n" +
		"./lib.sh: line 4: nosuch-lib-zz: not found\n" +
		"./lib.sh: line 2: nosuch-in-f-zz: not found\n" +
		"build.sh: line 5: cannot open missing: no such file\n"
	if got := runNamed(t, "build.sh", true, script); got != want {
		t.Errorf("stderr\n%s\nwant\n%s", got, want)
	}
}

// A command string has no file, so it is $0's name: nemosh's own unless an operand after the
// string gave one, as bash's `-c` takes it.
func TestDiagnosticPrefix_aCommandStringIsNamedByDollarZero(t *testing.T) {
	if got, want := runNamed(t, "probe", false, "\necho ${v?}\n"), "probe: line 2: v: parameter not set\n"; got != want {
		t.Errorf("stderr %q, want %q", got, want)
	}
}

// At a prompt the line is the one just typed, so there is none, and the name is the shell's.
func TestDiagnosticPrefix_aPromptSaysTheShellsNameAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	rt.SetArguments("build.sh", nil)
	rt.MarkSession()
	rt.RunScript(context.Background(), "\ncd nosuch-dir-zz\n")
	if got, want := stderr.String(), "nemosh: cd: cannot cd to nosuch-dir-zz: No such file or directory\n"; got != want {
		t.Errorf("stderr %q, want %q", got, want)
	}
}

// A script that does not parse is named without a line: the parse found the error, not a
// command running on one, and its message says where.
func TestDiagnosticPrefix_aParseErrorNamesTheScriptAlone(t *testing.T) {
	got := runNamed(t, "build.sh", true, "echo a\nif true; then\n")
	if want := "build.sh: incomplete script: missing fi for compound at line 2\n"; got != want {
		t.Errorf("stderr %q, want %q", got, want)
	}
}
