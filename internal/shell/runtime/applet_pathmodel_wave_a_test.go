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

func TestP05WaveA_FilesystemApplets_useTmpBackingAndCanonicalDisplay(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpRoot, "input.txt"), []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	settings := runtime.DefaultPathSettings()
	settings.TmpRoot = runtime.WorkingDirectory(tmpRoot)
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd:   runtime.WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(), "cat /tmp/input.txt\ncp /tmp/input.txt /tmp/copy.txt\nmkdir /tmp/dir\nmv /tmp/copy.txt /tmp/dir/moved.txt\nls /tmp/dir\nfind /tmp/dir\nrealpath /tmp/dir/moved.txt\nwinpath /tmp/input.txt\nposixpath /mnt/c/example.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	want := "alpha\nbeta\nmoved.txt\n/tmp/dir\n/tmp/dir/moved.txt\n/tmp/dir/moved.txt\n" + filepath.ToSlash(filepath.Join(tmpRoot, "input.txt")) + "\n/c/example.txt\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected canonical applet output %q, got %q", want, got)
	}
}

func TestP05WaveA_FilesystemApplets_propagatePathmodelErrors(t *testing.T) {
	// Given
	commands := []string{
		"cat /cygdrive/c/input.txt",
		"touch /cygdrive/c/output.txt",
		"ls /cygdrive/c",
		"cp /cygdrive/c/source /tmp/dest",
		"find /cygdrive/c",
		"realpath /cygdrive/c/input.txt",
	}
	for _, command := range commands {
		t.Run(strings.Fields(command)[0], func(t *testing.T) {
			var stderr bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr}, runtime.State{Cwd: runtime.WorkingDirectory(t.TempDir())})

			// When
			status := rt.RunScript(context.Background(), command+"\n")

			// Then
			if status == 0 {
				t.Fatalf("expected %q to fail", command)
			}
			if output := stderr.String(); !strings.Contains(output, "cygdrive paths are disabled") {
				t.Fatalf("expected pathmodel error for %q, got %q", command, output)
			}
		})
	}
}

func TestP05WaveA_LnSymbolic_preservesTargetPayload(t *testing.T) {
	if os.PathSeparator != '\\' {
		t.Skip("Windows pathmodel integration")
	}

	// Given
	tmpRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpRoot, "dir"), 0o700); err != nil {
		t.Fatalf("create link directory: %v", err)
	}
	settings := runtime.DefaultPathSettings()
	settings.TmpRoot = runtime.WorkingDirectory(tmpRoot)
	var stdout, stderr bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr}, runtime.State{
		Cwd:   runtime.WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(), "ln -s ../target /tmp/dir/link\nreadlink /tmp/dir/link\n")

	// Then
	if status != 0 {
		message := strings.ToLower(stderr.String())
		if strings.Contains(message, "privilege") || strings.Contains(message, "permission") {
			t.Skipf("symlink permission unavailable: %s", stderr.String())
		}
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got := filepath.ToSlash(strings.TrimSpace(stdout.String())); got != "../target" {
		t.Fatalf("expected symlink payload %q, got %q", "../target\n", got)
	}
}
