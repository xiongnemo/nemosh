//go:build windows

package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// runFrom runs a script with cwd as the shell's working directory.
func runFrom(t *testing.T, cwd, script string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := shellruntime.NewWithState(applets.DefaultRegistry, shellruntime.Streams{Stdout: &stdout, Stderr: &stderr},
		shellruntime.State{Cwd: shellruntime.WorkingDirectory(cwd)})
	status := rt.RunScript(context.Background(), script)
	return status, stdout.String(), stderr.String()
}

// nemoshSpelling is the drive-letter path a native path is written as inside
// Nemosh: C:\dir becomes /c/dir.
func nemoshSpelling(native string) string {
	return "/" + strings.ToLower(native[:1]) + filepath.ToSlash(native[2:])
}

// Nemosh's cwd is a value in pathState rather than the process's, so it is not
// bound by SetCurrentDirectory's MAX_PATH -- which is exactly what stops
// busybox-w32 from reaching here at all (win32/mingw.c:1703). Everything past
// the shell goes through Go's os package, which applies the extended-length
// prefix itself, so applets and redirection follow for free. The one Win32
// boundary the prefix cannot widen is a child's working directory;
// external_directory.go carries that, and external_directory_windows_test.go
// pins it.
func TestRuntime_readsAndWritesFilesPastMaxPath(t *testing.T) {
	// Given a directory well past MAX_PATH holding a file.
	deep := directoryOfWideLength(t, 310)
	if err := os.WriteFile(filepath.Join(deep, "read.txt"), []byte("payload\n"), 0o600); err != nil {
		t.Fatalf("seed the deep file: %v", err)
	}
	slash := filepath.ToSlash(deep)

	for _, testCase := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "an applet reads a file below it",
			script: "cat " + slash + "/read.txt\n",
			want:   "payload\n",
		},
		{
			name:   "a redirection creates a file below it, and reads it back",
			script: "echo written > " + slash + "/write.txt\ncat " + slash + "/write.txt\n",
			want:   "written\n",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runFrom(t, t.TempDir(), testCase.script)

			// Then
			if status != 0 {
				t.Fatalf("status=%d stderr=%q", status, stderr)
			}
			if stdout != testCase.want {
				t.Errorf("got %q, want %q", stdout, testCase.want)
			}
		})
	}
}

func TestRuntime_entersDirectoriesPastMaxPath(t *testing.T) {
	// Given
	deep := directoryOfWideLength(t, 310)

	// When
	status, stdout, stderr := runFrom(t, t.TempDir(), "cd "+filepath.ToSlash(deep)+"\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("status=%d stderr=%q", status, stderr)
	}
	if want := nemoshSpelling(deep) + "\n"; stdout != want {
		t.Errorf("pwd printed %q, want %q", stdout, want)
	}
}

// TestRuntime_launchesAChildFromAnImagePathPastMaxPath fills the gap the
// coverage above left: nothing here launched anything, so the claim that long
// paths hold at the launch boundary rested on the filesystem half alone.
//
// Measured on Windows 10 19045, the image path is not the boundary the child's
// working directory is. Go's os/exec reaches CreateProcess with a wide image
// path and an 804-character one starts, so there is nothing for Nemosh to
// widen. The child's cwd is the one that is MAX_PATH-bound, and
// external_directory.go already answers for it.
func TestRuntime_launchesAChildFromAnImagePathPastMaxPath(t *testing.T) {
	// Given a copy of a real program sitting well past MAX_PATH.
	source := filepath.Join(os.Getenv("SystemRoot"), "System32", "hostname.exe")
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("no host program to copy: %v", err)
	}
	deep := directoryOfWideLength(t, 310)
	image := filepath.Join(deep, "child.exe")
	if err := os.WriteFile(image, payload, 0o700); err != nil {
		t.Fatalf("seed the deep program: %v", err)
	}

	// When
	status, stdout, stderr := runFrom(t, t.TempDir(), "'"+filepath.ToSlash(image)+"'\n")

	// Then
	if status != 0 {
		t.Fatalf("status=%d stderr=%q (image is %d characters)", status, stderr, len(image))
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("the child produced no output; stderr=%q", stderr)
	}
}

// A child cannot be given a working directory past MAX_PATH unless the volume
// offers an 8.3 short name, and the diagnostic has to say so in those words:
// Windows reports "The directory name is invalid", which says nothing about
// length, and on a volume with 8.3 generation switched off there is no way to
// launch from here at all.
func TestRuntime_reportsTheLength_whenAChildsWorkingDirectoryCannotBeShortened(t *testing.T) {
	// Given a working directory far past anything an 8.3 name could rescue.
	// Nested rather than one long component, because a single name is capped at
	// 255 characters no matter how long the path may be.
	deep := t.TempDir()
	for len(deep) < 700 {
		deep = filepath.Join(deep, strings.Repeat("d", 24))
	}
	if err := os.MkdirAll(deep, 0o777); err != nil {
		t.Fatalf("create the nested directory: %v", err)
	}
	source := filepath.Join(os.Getenv("SystemRoot"), "System32", "hostname.exe")
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("no host program to copy: %v", err)
	}
	image := filepath.Join(deep, "child.exe")
	if err := os.WriteFile(image, payload, 0o700); err != nil {
		t.Fatalf("seed the deep program: %v", err)
	}

	// When
	status, _, stderr := runFrom(t, deep, "./child.exe\n")

	// Then
	if status == 0 {
		t.Skip("this volume shortened a 700-character working directory")
	}
	if !strings.Contains(stderr, "working directory is too long") {
		t.Fatalf("stderr=%q, want it to name the length as the constraint", stderr)
	}
}
