//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestNewRuntimeWithState_rejectsRelativeAmbientTmpRoot(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("TMPDIR", "relative-tmp")
	if tmpRoot := os.TempDir(); filepath.IsAbs(tmpRoot) {
		t.Fatalf("ambient temp root: got absolute %q, want relative fixture", tmpRoot)
	}

	_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd: WorkingDirectory(cwd),
	})

	if !errors.Is(err, ErrStateTmpRootNotAbsolute) {
		t.Fatalf("construct runtime: got %v, want %v", err, ErrStateTmpRootNotAbsolute)
	}
}

func TestNewRuntime_rejectsRelativeAmbientTmpRoot(t *testing.T) {
	t.Setenv("TMPDIR", "relative-tmp")

	_, err := NewRuntime(applets.DefaultRegistry, Streams{})

	if !errors.Is(err, ErrStateTmpRootNotAbsolute) {
		t.Fatalf("construct host runtime: got %v, want %v", err, ErrStateTmpRootNotAbsolute)
	}
}

func TestNew_preservesCompatibilityWhenAmbientTmpRootIsRelative(t *testing.T) {
	target := filepath.Join(t.TempDir(), "should-not-exist")
	t.Setenv("TMPDIR", "relative-tmp")
	var stderr bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

	status := runtime.RunScript(context.Background(), "echo effect >"+filepath.ToSlash(target)+"\n")

	if status != 1 {
		t.Fatalf("status: got %d, want 1", status)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("effect path: got %v, want not exist", err)
	}
	if !strings.Contains(stderr.String(), ErrStateTmpRootNotAbsolute.Error()) {
		t.Fatalf("stderr: %q", stderr.String())
	}
}
