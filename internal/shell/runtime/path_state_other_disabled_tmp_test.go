//go:build !windows

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestPathStateOther_disabledTmpKeepsNativeCwdAndRelativeResolution(t *testing.T) {
	nativeDir, err := os.MkdirTemp("/tmp", "nemosh-disabled-tmp-")
	if err != nil {
		t.Fatalf("create native tmp fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(nativeDir) })
	poisonRoot := t.TempDir()
	settings := DefaultPathSettings()
	settings.Config.EnableTmp = false
	settings.TmpRoot = WorkingDirectory(poisonRoot)
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	status := runtime.RunScript(context.Background(), "cd "+nativeDir+"\necho native > child.txt\n")
	nativeCwd, nativeErr := runtime.nativeWorkingDirectory()
	resolved, resolveErr := runtime.ResolveNemoshPath("child.txt")

	if status != 0 || nativeErr != nil || resolveErr != nil {
		t.Fatalf("disabled tmp execution: status=%d nativeErr=%v resolveErr=%v", status, nativeErr, resolveErr)
	}
	if nativeCwd != nativeDir || !filepath.IsAbs(nativeCwd) {
		t.Fatalf("native cwd: got %q, want absolute %q", nativeCwd, nativeDir)
	}
	if resolved.Native != filepath.Join(nativeDir, "child.txt") || !filepath.IsAbs(resolved.Native) {
		t.Fatalf("resolved path: got %+v, want native child under %q", resolved, nativeDir)
	}
	assertPathFileText(t, filepath.Join(nativeDir, "child.txt"), "native\n")
	if _, statErr := os.Stat(filepath.Join(poisonRoot, filepath.Base(nativeDir), "child.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("poison backing effect: got %v, want not exist", statErr)
	}
}

func TestPathStateOther_disabledTmpDoesNotCanonicalizeThroughTmpRoot(t *testing.T) {
	settings := DefaultPathSettings()
	settings.Config.EnableTmp = false
	settings.TmpRoot = WorkingDirectory(t.TempDir())
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})
	native := filepath.Join(string(settings.TmpRoot), "generated.txt")

	canonical, err := runtime.CanonicalizeNativePath(pathmodel.Path("/tmp/source"), native)

	if err != nil {
		t.Fatalf("canonicalize disabled tmp path: %v", err)
	}
	want := pathmodel.Path(filepath.ToSlash(filepath.Clean(native)))
	if canonical != want {
		t.Fatalf("canonical path: got %q, want %q", canonical, want)
	}
}
