//go:build windows

package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A session started straight from Windows Terminal inherits none of the
// variables a shell assumes it was logged in with, and the symptoms do not look
// like one cause: a literal tilde, `cd` refusing, no history saved, no ssh host
// completed, and a busybox ash started inside unable to source ~/.profile.
//
// Measured before the fix, with HOME removed from the environment:
//
//	$ echo ~        -> C:/Users/nemo   (the USERPROFILE fallback, in the host's
//	                                    spelling, disagreeing with pwd)
//	$ cd            -> cd: HOME not set
func TestRuntime_fillsInTheLoginVariablesWindowsDoesNotSet(t *testing.T) {
	// Given: an environment with none of them, which is what Windows provides
	var stdout strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment([]string{`SystemRoot=C:\Windows`})})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if status := rt.RunScript(t.Context(), "echo $HOME\necho $USER\necho $LOGNAME\ncd\npwd\necho ~\n"); status != 0 {
		t.Fatalf("status = %d, output %q", status, stdout.String())
	}

	// Then
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	// Five, not six: `cd` prints nothing when it succeeds.
	if len(lines) != 5 {
		t.Fatalf("output = %q, want five lines", stdout.String())
	}
	home := lines[0]
	if home == "" {
		t.Fatal("HOME is still empty")
	}
	// This shell's own spelling, like every other path it reports, so that
	// `echo $HOME`, `echo ~` and `pwd` all say the same thing about the same
	// directory.
	//
	// It briefly was not. Storing it natively made those three agree with a
	// launched program and disagree with each other; the fix belongs at the one
	// place the two worlds meet, not in the value. See environment_launch.go and
	// TestRuntime_translatesItsOwnPathVariablesForALaunchedProgram.
	if !strings.HasPrefix(home, "/") {
		t.Fatalf("HOME = %q, want the shell's own spelling", home)
	}
	if lines[1] == "" || lines[1] != lines[2] {
		t.Fatalf("USER = %q, LOGNAME = %q, want both set and equal", lines[1], lines[2])
	}
	// `cd` with no operand goes home, and pwd says exactly what $HOME says --
	// which is the point of keeping one spelling inside.
	if lines[3] != home {
		t.Fatalf("pwd after cd = %q, want %q", lines[3], home)
	}
	if lines[4] != home {
		t.Fatalf("echo ~ = %q, want %q", lines[4], home)
	}
}

// Only if unset. An environment that says where home is has said so on purpose.
func TestRuntime_doesNotOverrideAnInheritedHome(t *testing.T) {
	// Given
	var stdout strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment([]string{"HOME=/c/elsewhere", "USER=someone"})})
	if err != nil {
		t.Fatal(err)
	}

	// When
	if status := rt.RunScript(t.Context(), "echo $HOME\necho $USER\n"); status != 0 {
		t.Fatalf("status = %d", status)
	}

	// Then
	if got := stdout.String(); got != "/c/elsewhere\nsomeone\n" {
		t.Fatalf("output = %q, want the inherited values untouched", got)
	}
}

// Exported, not merely set -- which is the whole reason ash could not find
// ~/.profile.
//
// `env` reports the shell's environment, in the shell's spelling. What a
// launched program actually receives is translated at the boundary and is
// asserted against a real child in
// TestRuntime_translatesItsOwnPathVariablesForALaunchedProgram; the two are
// different questions and this is the first one.
func TestRuntime_exportsTheFilledInHome(t *testing.T) {
	// Given
	var stdout strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(nil)})
	if err != nil {
		t.Fatal(err)
	}

	// When: env lists what a child would receive
	if status := rt.RunScript(t.Context(), "env | grep HOME=\n"); status != 0 {
		t.Fatalf("status = %d, output %q", status, stdout.String())
	}

	// Then
	if exported := strings.TrimSpace(stdout.String()); !strings.HasPrefix(exported, "HOME=/") {
		t.Fatalf("env showed %q, want an exported HOME in the shell's spelling", exported)
	}
}

// The launch boundary is where the two spellings meet, and the only place.
//
// Inside, every path this shell reports uses its own form. Outside is a native
// program that has never heard of it -- measured, with HOME exported unchanged:
//
//	busybox ash -c 'cd $HOME; pwd'  ->  C:/c/Users/nemo
//
// Before this, the environment handed out both forms at once: PWD as /c/... and
// OLDPWD as C:/..., in the same block. That is not a rule, it is the absence of
// one.
func TestRuntime_translatesItsOwnPathVariablesForALaunchedProgram(t *testing.T) {
	// Given
	var stdout, stderr strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(nil)})
	if err != nil {
		t.Fatal(err)
	}

	// When: cmd.exe reports what it was handed, which no amount of shell
	// bookkeeping can influence.
	// Two echoes rather than one with a separator: a `|` in the script is a
	// pipe, which is how the first version of this test came to report
	// `%PWD%: not found`.
	status := rt.RunScript(t.Context(), commandProcessor(t)+" /c echo %HOME%\n"+commandProcessor(t)+" /c echo %PWD%\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, output %q, stderr %q", status, stdout.String(), stderr.String())
	}
	for _, handed := range strings.Fields(stdout.String()) {
		if strings.HasPrefix(handed, "/") {
			t.Fatalf("the child was handed %q, want a native path it can open", handed)
		}
	}
	// And the shell still says its own thing about the same directories.
	stdout.Reset()
	if status := rt.RunScript(t.Context(), "echo $HOME\n"); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if inside := strings.TrimSpace(stdout.String()); !strings.HasPrefix(inside, "/") {
		t.Fatalf("inside the shell HOME = %q, want the shell's own spelling", inside)
	}
}

// A variable the user exported travels verbatim, whatever it looks like. This is
// the whole difference from MSYS2, which rewrites anything resembling a path on
// its way out and is regularly wrong about arguments that were never paths.
func TestRuntime_doesNotRewriteVariablesItDidNotSet(t *testing.T) {
	// Given
	var stdout strings.Builder
	rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
		shellruntime.Streams{Stdout: &stdout},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(nil)})
	if err != nil {
		t.Fatal(err)
	}

	// When
	script := "export MYDIR=/c/Users/nemo\n" + commandProcessor(t) + " /c echo %MYDIR%\n"
	if status := rt.RunScript(t.Context(), script); status != 0 {
		t.Fatalf("status = %d, output %q", status, stdout.String())
	}

	// Then
	if got := strings.TrimSpace(stdout.String()); got != "/c/Users/nemo" {
		t.Fatalf("the child was handed %q, want it untouched", got)
	}
}

// commandProcessor is cmd.exe by absolute path, because these runtimes are built
// with an empty environment on purpose -- the point is what the shell puts into
// a child's, and an inherited PATH would be one more thing in the way.
func commandProcessor(t *testing.T) string {
	t.Helper()
	root := os.Getenv("SystemRoot")
	if root == "" {
		t.Skip("no SystemRoot to find cmd.exe with")
	}
	return filepath.ToSlash(filepath.Join(root, "System32", "cmd.exe"))
}
