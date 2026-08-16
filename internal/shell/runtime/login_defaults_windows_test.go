//go:build windows

package runtime_test

import (
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
	// The shell's own spelling, not the host's. Otherwise `echo $HOME` and
	// `pwd` disagree about the same directory, and a script comparing them is
	// wrong for a reason nobody would guess.
	if !strings.HasPrefix(home, "/") {
		t.Fatalf("HOME = %q, want the shell's spelling rather than a native path", home)
	}
	if lines[1] == "" || lines[1] != lines[2] {
		t.Fatalf("USER = %q, LOGNAME = %q, want both set and equal", lines[1], lines[2])
	}
	// `cd` with no operand goes home, and `pwd` agrees with $HOME.
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

// And it has to reach the programs started from here, which is the whole reason
// ash could not find ~/.profile.
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
	if !strings.HasPrefix(stdout.String(), "HOME=/") {
		t.Fatalf("env showed %q, want an exported HOME", stdout.String())
	}
}
