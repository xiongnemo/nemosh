package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestDirectPathApplets_preserveAllPlatformLexicalContracts(t *testing.T) {
	tests := []struct {
		name   string
		applet string
		input  string
		want   string
	}{
		{name: "posix drive", applet: "posixpath", input: "C:/a/nonexistent", want: "/c/a/nonexistent\n"},
		{name: "posix backslash", applet: "posixpath", input: `C:\a\nonexistent`, want: "/c/a/nonexistent\n"},
		{name: "posix UNC", applet: "posixpath", input: "//server/share/nonexistent", want: "//server/share/nonexistent\n"},
		{name: "win short drive", applet: "winpath", input: "/c/a/nonexistent", want: "C:/a/nonexistent\n"},
		{name: "win mount alias", applet: "winpath", input: "/mnt/c/a/nonexistent", want: "C:/a/nonexistent\n"},
		{name: "win native drive", applet: "winpath", input: "C:/a/nonexistent", want: "C:/a/nonexistent\n"},
		{name: "win backslash", applet: "winpath", input: `C:\a\nonexistent`, want: "C:/a/nonexistent\n"},
		{name: "win UNC", applet: "winpath", input: "//server/share/nonexistent", want: "//server/share/nonexistent\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, args := range [][]string{{"nemosh", test.applet, test.input}, {directAppletInvocationName(test.applet), test.input}} {
				// When
				stdout, stderr, err := runDirectAppletTest(args)

				// Then
				if err != nil || stdout != test.want || stderr != "" {
					t.Fatalf("run(%v): stdout=%q stderr=%q error=%v, want %q", args, stdout, stderr, err, test.want)
				}
			}
		})
	}
}

func TestDirectPathApplets_useTypedTmpIdentityAndBacking(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	settings := shellruntime.DefaultPathSettings()
	settings.TmpRoot = shellruntime.WorkingDirectory(tmpRoot)
	state := shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(os.Environ()), Paths: &settings}
	native := filepath.ToSlash(filepath.Join(tmpRoot, "nonexistent")) + "\n"

	tests := []struct {
		applet string
		want   string
	}{
		{applet: "posixpath", want: "/tmp/nonexistent\n"},
		{applet: "winpath", want: native},
	}
	for _, test := range tests {
		for _, args := range [][]string{{"nemosh", test.applet, "/tmp/nonexistent"}, {directAppletInvocationName(test.applet), "/tmp/nonexistent"}} {
			// When
			stdout, stderr, err := runDirectAppletStateTest(args, state)

			// Then
			if err != nil || stdout != test.want || stderr != "" {
				t.Fatalf("run(%v): stdout=%q stderr=%q error=%v, want %q", args, stdout, stderr, err, test.want)
			}
		}
	}
}

func TestDirectPathApplets_preserveTypedCygdrivePolicy(t *testing.T) {
	// Given
	settings := shellruntime.DefaultPathSettings()
	settings.Config.AcceptCygdrive = true
	state := shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(nil), Paths: &settings}

	for _, applet := range []string{"posixpath", "winpath"} {
		// When
		stdout, stderr, err := runDirectAppletStateTest([]string{"nemosh", applet, "/cygdrive/c/nonexistent"}, state)

		// Then
		if err != nil || stdout != map[string]string{"posixpath": "/c/nonexistent\n", "winpath": "C:/nonexistent\n"}[applet] || stderr != "" {
			t.Fatalf("%s: stdout=%q stderr=%q error=%v", applet, stdout, stderr, err)
		}
	}

	stdout, stderr, err := runDirectAppletTest([]string{"nemosh", "posixpath", "/cygdrive/c/nonexistent"})
	if !errors.Is(err, applets.ErrExitFalse) || stdout != "" || stderr != "cygdrive paths are disabled\n" {
		t.Fatalf("disabled Cygdrive: stdout=%q stderr=%q error=%v", stdout, stderr, err)
	}
}
