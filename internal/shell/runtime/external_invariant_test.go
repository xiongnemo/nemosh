package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestExecutableCandidate_rejectsRelativeNativePath(t *testing.T) {
	t.Chdir(t.TempDir())
	candidate := "relative-probe"
	if os.PathSeparator == '\\' {
		candidate += ".exe"
	}
	if err := os.WriteFile(candidate, []byte("probe"), 0o700); err != nil {
		t.Fatalf("write relative candidate: %v", err)
	}

	_, err := executableCandidate(candidate)

	if !errors.Is(err, errExternalPathNotAbsolute) {
		t.Fatalf("candidate error: got %v, want %v", err, errExternalPathNotAbsolute)
	}
}

func TestRequireAbsoluteNativePath_acceptsAbsolutePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe")

	got, err := requireAbsoluteNativePath("executable", path)

	if err != nil {
		t.Fatalf("absolute path: %v", err)
	}
	if got != path {
		t.Fatalf("absolute path: got %q, want %q", got, path)
	}
}

func TestExecutableCandidate_preservesFilesystemError(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "invalid\x00name")

	_, err := executableCandidate(candidate)

	if err == nil {
		t.Fatal("candidate error: got nil, want filesystem error")
	}
	if errors.Is(err, errExternalNotFound) {
		t.Fatalf("candidate error: got %v, want non-not-found filesystem error", err)
	}
}

func TestExecutableCandidate_reportsExistingDirectoryAsNotExecutable(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "directory.exe")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create directory candidate: %v", err)
	}

	_, err := executableCandidate(directory)

	if !errors.Is(err, errExternalNotExecutable) {
		t.Fatalf("candidate error: got %v, want %v", err, errExternalNotExecutable)
	}
}

func TestExternalCommandPath_continuesAfterCandidateError_andFindsLaterExecutable(t *testing.T) {
	command := "probe"
	if os.PathSeparator == '\\' {
		command += ".exe"
	}
	validDir := t.TempDir()
	valid := filepath.Join(validDir, command)
	if err := os.WriteFile(valid, []byte("probe"), 0o700); err != nil {
		t.Fatalf("write valid candidate: %v", err)
	}
	invalidDir := filepath.Join(t.TempDir(), "invalid\x00dir")
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment([]string{"PATH=" + invalidDir + string(os.PathListSeparator) + validDir}),
	})

	got, err := runtime.externalCommandPath(command)

	if err != nil {
		t.Fatalf("lookup candidate: %v", err)
	}
	if got != valid {
		t.Fatalf("candidate: got %q, want %q", got, valid)
	}
}

func TestExternalCommandPath_continuesAfterPathResolutionError_andFindsLaterExecutable(t *testing.T) {
	command := "probe"
	if os.PathSeparator == '\\' {
		command += ".exe"
	}
	validDir := t.TempDir()
	valid := filepath.Join(validDir, command)
	if err := os.WriteFile(valid, []byte("probe"), 0o700); err != nil {
		t.Fatalf("write valid candidate: %v", err)
	}
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment([]string{"PATH=/cygdrive/c" + string(os.PathListSeparator) + validDir}),
	})

	got, err := runtime.externalCommandPath(command)

	if err != nil {
		t.Fatalf("lookup candidate after path resolution error: %v", err)
	}
	if got != valid {
		t.Fatalf("candidate: got %q, want %q", got, valid)
	}
}

func TestExternalCommandPath_returnsFirstPathResolutionError_whenLaterCandidatesMiss(t *testing.T) {
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment([]string{"PATH=/cygdrive/c" + string(os.PathListSeparator) + t.TempDir()}),
	})

	_, err := runtime.externalCommandPath("nemosh-missing-after-policy-error")

	if !errors.Is(err, pathmodel.ErrCygdriveDisabled) {
		t.Fatalf("lookup error: got %v, want %v", err, pathmodel.ErrCygdriveDisabled)
	}
}

func TestExternalCommandPath_reportsResolvedDeviceAsNotExecutable(t *testing.T) {
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment(nil),
	})

	_, err := runtime.externalCommandPath("/dev/null")

	if !errors.Is(err, errExternalNotExecutable) {
		t.Fatalf("device error: got %v, want %v", err, errExternalNotExecutable)
	}
}

func TestRuntime_explicitResolvedDeviceReturns126(t *testing.T) {
	var stderr bytes.Buffer
	runtime := NewWithState(applets.DefaultRegistry, Streams{Stderr: &stderr}, State{
		Cwd: WorkingDirectory(t.TempDir()),
		Env: NewEnvironment(nil),
	})

	status := runtime.RunScript(context.Background(), "/dev/null\n")

	if status != 126 {
		t.Fatalf("status: got %d, want 126", status)
	}
	if got := stderr.String(); !strings.Contains(got, "not executable") || strings.Contains(got, "not found") {
		t.Fatalf("stderr: got %q, want actionable non-not-found diagnostic", got)
	}
}

func TestExecutableCandidate_windowsContinuesAfterBadEarlierSuffix(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows suffix lookup")
	}
	base := filepath.Join(t.TempDir(), "probe")
	if err := os.Mkdir(base+".com", 0o700); err != nil {
		t.Fatalf("create bad .com candidate: %v", err)
	}
	want := base + ".exe"
	if err := os.WriteFile(want, []byte("probe"), 0o700); err != nil {
		t.Fatalf("write .exe candidate: %v", err)
	}

	got, err := executableCandidate(base)

	if err != nil {
		t.Fatalf("lookup suffix candidate: %v", err)
	}
	if got != want {
		t.Fatalf("candidate: got %q, want %q", got, want)
	}
}
