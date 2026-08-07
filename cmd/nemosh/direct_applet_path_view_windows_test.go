//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestDirectApplet_pwdUsesCanonicalWindowsCWD_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	// Given
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	want, err := pathmodel.New(pathmodel.DefaultConfig(), "/c").Resolve(filepath.ToSlash(cwd))
	if err != nil {
		t.Fatalf("canonicalize cwd: %v", err)
	}

	for _, args := range [][]string{{"nemosh", "pwd"}, {"pwd.exe"}} {
		// When
		stdout, _, runErr := runDirectAppletTest(args)

		// Then
		if runErr != nil || stdout != string(want)+"\n" {
			t.Fatalf("run(%v): stdout=%q error=%v, want %q", args, stdout, runErr, want)
		}
	}
}

func TestDirectApplet_catReadsMountAlias_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("mount-data\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	alias := windowsAliasPath(t, "/mnt", path)

	for _, args := range [][]string{{"nemosh", "cat", alias}, {"cat.exe", alias}} {
		// When
		stdout, stderr, err := runDirectAppletTest(args)

		// Then
		if err != nil || stdout != "mount-data\n" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
}

func TestDirectApplet_disabledCygdriveFailsBeforeTouchEffect_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "touch"}, {"touch.exe"}} {
		// Given
		effect := filepath.Join(t.TempDir(), "effect.txt")
		invocation := append(args, windowsAliasPath(t, "/cygdrive", effect))

		// When
		stdout, stderr, err := runDirectAppletTest(invocation)

		// Then
		if !errors.Is(err, pathmodel.ErrCygdriveDisabled) {
			t.Fatalf("run(%v): error=%v, want ErrCygdriveDisabled", invocation, err)
		}
		if stdout != "" {
			t.Fatalf("run(%v): stdout=%q, want nothing", invocation, stdout)
		}
		// Reported under the applet's name rather than silently, which is what
		// direct dispatch used to do with every failure.
		if !strings.Contains(stderr, "touch: ") || !strings.Contains(stderr, "cygdrive") {
			t.Fatalf("run(%v): stderr=%q, want a touch-prefixed cygdrive diagnostic", invocation, stderr)
		}
		if _, statErr := os.Stat(effect); !os.IsNotExist(statErr) {
			t.Fatalf("effect exists after typed path error: %v", statErr)
		}
	}
}

func TestDirectApplet_realpathDisplaysCanonicalWindowsPath_whenSelectedExplicitlyOrByInvocationName(t *testing.T) {
	// Given a temporary directory in the spelling realpath will answer with:
	// on a machine whose TEMP sits under an 8.3 alias -- which is what GitHub's
	// Windows runners hand out, because the profile directory name is longer
	// than eight characters -- t.TempDir() is the short form and realpath
	// answers with the long one.
	path := filepath.Join(canonicalTempDirForDirectApplet(t), "input.txt")
	if err := os.WriteFile(path, []byte("realpath"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	alias := windowsAliasPath(t, "/mnt", path)
	want := strings.TrimPrefix(alias, "/mnt") + "\n"

	for _, args := range [][]string{{"nemosh", "realpath", alias}, {"realpath.exe", alias}} {
		// When
		stdout, stderr, err := runDirectAppletTest(args)

		// Then
		if err != nil || stdout != want || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v, want stdout=%q", args, stdout, stderr, err, want)
		}
	}
}

func TestDirectApplet_catReadsCygdriveAlias_whenEnabledForBothEntryForms(t *testing.T) {
	// Given
	cwd := t.TempDir()
	path := filepath.Join(cwd, "input.txt")
	if err := os.WriteFile(path, []byte("cygdrive-data\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	settings := shellruntime.DefaultPathSettings()
	settings.Config.AcceptCygdrive = true
	state := shellruntime.State{Cwd: shellruntime.WorkingDirectory(cwd), Env: shellruntime.NewEnvironment(os.Environ()), Paths: &settings}
	alias := windowsAliasPath(t, "/cygdrive", path)

	for _, args := range [][]string{{"nemosh", "cat", alias}, {"cat.exe", alias}} {
		// When
		stdout, stderr, err := runDirectAppletStateTest(args, state)

		// Then
		if err != nil || stdout != "cygdrive-data\n" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
}

func TestDirectApplet_envPreservesTypedPathViewForNestedAppletInBothEntryForms(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("nested-env-data\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	alias := windowsAliasPath(t, "/mnt", path)

	for _, args := range [][]string{{"nemosh", "env", "CHILD=value", "cat", alias}, {"env.exe", "CHILD=value", "cat", alias}} {
		// When
		stdout, stderr, err := runDirectAppletTest(args)

		// Then
		if err != nil || stdout != "nested-env-data\n" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
}

func TestDirectApplet_pwdDisplaysCanonicalUNCFromInjectedStateForBothEntryForms(t *testing.T) {
	// Given
	state := shellruntime.State{Cwd: `//server/share/dir`, Env: shellruntime.NewEnvironment(nil)}

	for _, args := range [][]string{{"nemosh", "pwd"}, {"pwd.exe"}} {
		// When
		stdout, stderr, err := runDirectAppletStateTest(args, state)

		// Then
		if err != nil || stdout != "//server/share/dir\n" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
}

func TestDirectApplet_posixpathDisplaysCanonicalAliasForBothEntryForms(t *testing.T) {
	for _, args := range [][]string{{"nemosh", "posixpath", "/mnt/c/example.txt"}, {"posixpath.exe", "/mnt/c/example.txt"}} {
		// When
		stdout, stderr, err := runDirectAppletTest(args)

		// Then
		if err != nil || stdout != "/c/example.txt\n" || stderr != "" {
			t.Fatalf("run(%v): stdout=%q stderr=%q error=%v", args, stdout, stderr, err)
		}
	}
}

func windowsAliasPath(t *testing.T, prefix, native string) string {
	t.Helper()
	canonical, err := pathmodel.New(pathmodel.DefaultConfig(), "/c").Resolve(filepath.ToSlash(native))
	if err != nil {
		t.Fatalf("canonicalize %q: %v", native, err)
	}
	return prefix + string(canonical)
}

// canonicalTempDirForDirectApplet is t.TempDir() resolved the way realpath
// resolves it, so a test can use one spelling for the file it creates and for
// the answer it expects.
func canonicalTempDirForDirectApplet(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}
