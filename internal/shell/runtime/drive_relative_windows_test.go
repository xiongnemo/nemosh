//go:build windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A drive-relative path is relative to where its drive was left, as busybox-w32 reads one;
// see pathmodel/drive_directories.go. These use the drive the temporary directory is on, which
// is the one drive a test can count on.

func driveRelativeRuntime(t *testing.T) (Runtime, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "file"), []byte("in sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr},
		State{Cwd: WorkingDirectory(dir), Env: NewEnvironment(nil)})
	return rt, &stdout, &stderr, dir[:2]
}

func TestRuntime_aDriveRelativePathIsRelativeToItsDrivesDirectory(t *testing.T) {
	rt, stdout, stderr, drive := driveRelativeRuntime(t)

	// When: the drive alone, a name on it, and its parent, from where the shell is
	script := "here=$(pwd)\n" +
		"cat " + drive + "sub/file\n" +
		"cd " + drive + "sub && [ \"$(pwd)\" = \"$here/sub\" ] && echo into sub\n" +
		"cd " + drive + ".. && [ \"$(pwd)\" = \"$here\" ] && echo back\n" +
		"cd " + drive + " && [ \"$(pwd)\" = \"$here\" ] && echo stayed\n"
	status := rt.RunScript(context.Background(), script)

	// Then
	if want := "in sub\ninto sub\nback\nstayed\n"; status != 0 || stdout.String() != want {
		t.Fatalf("status %d, stdout %q, stderr %q; want %q", status, stdout.String(), stderr.String(), want)
	}
}

// A launched program is handed where each drive was left, as Windows's `=D:` variables, so `D:`
// means to it what it means here.
func TestRuntime_aLaunchedProgramIsHandedEachDrivesDirectory(t *testing.T) {
	rt, _, _, drive := driveRelativeRuntime(t)
	if status := rt.RunScript(context.Background(), "cd sub\n"); status != 0 {
		t.Fatalf("cd sub: %d", status)
	}
	here, err := rt.paths.nativeWorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	other := "Q:/elsewhere"
	if strings.EqualFold(drive, "Q:") {
		other = "R:/elsewhere"
	}
	resolved, err := rt.ResolveNemoshPath(other)
	if err != nil {
		t.Fatal(err)
	}
	rt.paths.setWorkingDirectory(resolved)

	// When
	environment := rt.childEnvironment()

	// Then: both drives, at the front, in Windows's spelling
	wantHere := "=" + strings.ToUpper(drive) + "=" + filepath.FromSlash(here)
	wantOther := "=" + strings.ToUpper(other[:2]) + "=" + filepath.FromSlash(other)
	for _, want := range []string{wantHere, wantOther} {
		if !slices.Contains(environment, want) {
			t.Fatalf("the environment has no %q: %q", want, environment[:min(5, len(environment))])
		}
	}
	if !strings.HasPrefix(environment[0], "=") || !strings.HasPrefix(environment[1], "=") {
		t.Fatalf("the drives are not first: %q", environment[:2])
	}
}

// `cd C:\Users` unquoted reaches cd as C:Users. At a prompt, a failure says what may have
// happened to the backslash; in a script, as in busybox-w32, it says nothing more.
func TestRuntime_cdOfAnEatenBackslashHintsAtAPromptOnly(t *testing.T) {
	for _, session := range []bool{false, true} {
		rt, _, stderr, drive := driveRelativeRuntime(t)
		if session {
			rt.MarkSession()
		}

		// When
		rt.RunScript(context.Background(), "cd "+drive+"nosuch\n")

		// Then
		hinted := strings.Contains(stderr.String(), "hint: a backslash outside quotes is an escape")
		if !strings.Contains(stderr.String(), "cannot cd to "+drive+"nosuch") || hinted != session {
			t.Fatalf("session %v: stderr %q", session, stderr.String())
		}
	}
}
