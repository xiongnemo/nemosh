//go:build !windows

package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestResolveNemoshPath_rejectsRelativeNativePathFromPathState(t *testing.T) {
	settings := DefaultPathSettings()
	paths := newPathState(".", settings)
	runtime := newRuntimeWithState(applets.DefaultRegistry, Streams{}, State{}, paths, nil)

	_, err := runtime.ResolveNemoshPath("relative")

	if !errors.Is(err, ErrResolvedNativePathNotAbsolute) {
		t.Fatalf("resolve malformed path state: got %v, want %v", err, ErrResolvedNativePathNotAbsolute)
	}
}

func TestResolveNemoshPath_returnsAbsoluteNativePathForSuccessfulNonDeviceResults(t *testing.T) {
	settings := DefaultPathSettings()
	settings.Config.MountPrefix = "/volumes"
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})
	for _, input := range []string{"relative", "/etc/hostname", "/tmp/file", "/volumes/c/file"} {
		resolved, err := runtime.ResolveNemoshPath(input)
		if err != nil {
			t.Fatalf("resolve %q: %v", input, err)
		}
		if resolved.Device || !filepath.IsAbs(resolved.Native) {
			t.Fatalf("resolve %q: got %+v, want non-device absolute native path", input, resolved)
		}
	}
	device, err := runtime.ResolveNemoshPath("/dev/null")
	if err != nil || !device.Device || device.Native != "" {
		t.Fatalf("resolve device: got %+v error=%v", device, err)
	}
}

func TestNewWithState_relativeMountPrefixFailsBeforeScriptEffect(t *testing.T) {
	hostCwd := t.TempDir()
	backing := filepath.Join(hostCwd, "mnt", "c")
	if err := os.MkdirAll(backing, 0o700); err != nil {
		t.Fatalf("create host mount fixture: %v", err)
	}
	t.Chdir(hostCwd)
	settings := DefaultPathSettings()
	settings.Config.MountPrefix = "mnt"
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	status := runtime.RunScript(context.Background(), "echo effect > mnt/c/effect\n")

	if status != 1 {
		t.Fatalf("status: got %d, want 1", status)
	}
	if _, err := os.Stat(filepath.Join(backing, "effect")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("effect path: got %v, want not exist", err)
	}
	_, resolveErr := runtime.ResolveNemoshPath("mnt/c/effect")
	if !errors.Is(resolveErr, ErrStateMountPrefixNotAbsolute) {
		t.Fatalf("resolve invalid mount prefix: got %v, want %v", resolveErr, ErrStateMountPrefixNotAbsolute)
	}
}

func TestPathStateOther_relativeMountPrefixProducesRelativeNativePathBeforeBoundary(t *testing.T) {
	settings := DefaultPathSettings()
	settings.Config.MountPrefix = "mnt"
	state := newPathState(WorkingDirectory(t.TempDir()), settings)

	resolved, err := state.resolve("mnt/c/effect")

	if err != nil {
		t.Fatalf("resolve internal mount path: %v", err)
	}
	if resolved.Canonical != pathmodel.Path("/c/effect") || filepath.IsAbs(resolved.Native) {
		t.Fatalf("internal resolution: got %+v, want canonical /c/effect with relative native repro", resolved)
	}
}
