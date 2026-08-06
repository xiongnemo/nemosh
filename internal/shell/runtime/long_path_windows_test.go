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
