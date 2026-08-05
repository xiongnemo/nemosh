package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestP05WaveA_CD_preservesVirtualTmpSpellingAndBacking(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(tmpRoot)
	var stdout bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(), "cd /tmp\npwd\necho contents > child.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "/tmp\n" {
		t.Fatalf("expected virtual pwd %q, got %q", "/tmp\n", got)
	}
	assertPathFileText(t, filepath.Join(tmpRoot, "child.txt"), "contents\n")
}

func TestP05WaveA_SourceAndRedirect_useInjectedTmpBacking(t *testing.T) {
	// Given
	tmpRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpRoot, "setup.sh"), []byte("value=from-tmp\n"), 0o600); err != nil {
		t.Fatalf("write source fixture: %v", err)
	}
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(tmpRoot)
	rt := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	// When
	status := rt.RunScript(context.Background(), ". /tmp/setup.sh\necho $value > /tmp/output.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	assertPathFileText(t, filepath.Join(tmpRoot, "output.txt"), "from-tmp\n")
}

func TestP05WaveA_ShellIO_acceptsMountAliasAndDriveCurrentRoot(t *testing.T) {
	if !isWindowsRuntime() {
		t.Skip("Windows path aliases")
	}

	// Given
	nativeRoot := filepath.VolumeName(t.TempDir()) + string(os.PathSeparator)
	nativeDir := t.TempDir()
	canonicalDir := canonicalWindowsPath(t, nativeDir)
	settings := DefaultPathSettings()
	mountDir := settings.Config.MountPrefix + string(canonicalDir)
	var stdout, stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
		Cwd: WorkingDirectory(nativeDir),
	})

	// When
	status := rt.RunScript(context.Background(), "cd "+mountDir+"\necho mounted > mounted.txt\ncd /\npwd\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	assertPathFileText(t, filepath.Join(nativeDir, "mounted.txt"), "mounted\n")
	if got, want := stdout.String(), string(canonicalWindowsPath(t, nativeRoot))+"\n"; got != want {
		t.Fatalf("expected current-root pwd %q, got %q", want, got)
	}
}

func TestP05WaveA_CD_reportsHostOnlyUNCHintWithoutChangingCWD(t *testing.T) {
	if !isWindowsRuntime() {
		t.Skip("Windows UNC roots")
	}

	// Given
	cwd := t.TempDir()
	var stderr bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stderr: &stderr}, State{Cwd: WorkingDirectory(cwd)})
	wantCWD := rt.WorkingDirectory()

	// When
	status := rt.RunScript(context.Background(), "cd //server\n")

	// Then
	if status == 0 {
		t.Fatal("expected host-only UNC failure")
	}
	if got := rt.WorkingDirectory(); got != wantCWD {
		t.Fatalf("expected cwd to remain %q, got %q", wantCWD, got)
	}
	if output := stderr.String(); !strings.Contains(output, "//server is not a directory root") || !strings.Contains(output, "use //server/share") {
		t.Fatalf("expected targeted UNC hint, got %q", output)
	}
}

func TestP05WaveA_SourceAndRedirect_reportPathErrorBeforeEffects(t *testing.T) {
	// Given
	tests := []struct {
		name   string
		script string
	}{
		{name: "source", script: ". /cygdrive/c/setup.sh\n"},
		{name: "redirect", script: "echo should-not-run > /cygdrive/c/output.txt\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr}, State{
				Cwd: WorkingDirectory(t.TempDir()),
			})

			// When
			status := rt.RunScript(context.Background(), tt.script)

			// Then
			if status == 0 {
				t.Fatal("expected path resolution failure")
			}
			if !strings.Contains(stderr.String(), "cygdrive paths are disabled") {
				t.Fatalf("expected cygdrive error, got %q", stderr.String())
			}
			if tt.name == "redirect" && stdout.Len() != 0 {
				t.Fatalf("expected redirected command not to run, got stdout %q", stdout.String())
			}
			if tt.name == "source" && stdout.Len() != 0 {
				t.Fatalf("expected source failure to stop script, got stdout %q", stdout.String())
			}
		})
	}
}

func assertPathFileText(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got := string(data); got != want {
		t.Fatalf("expected %s contents %q, got %q", path, want, got)
	}
}

func isWindowsRuntime() bool {
	return os.PathSeparator == '\\'
}
