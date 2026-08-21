package runtime

import (
	"errors"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestP05WaveA_RuntimePathState_resolvesWindowsAliasesAndVirtualRoots(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("Windows path-model integration")
	}

	// Given
	cwd := t.TempDir()
	tmpRoot := t.TempDir()
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(tmpRoot)
	rt := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(cwd),
		Env:   NewEnvironment(nil),
		Paths: &settings,
	})
	canonicalCWD := canonicalWindowsPath(t, cwd)
	mountInput := settings.Config.MountPrefix + string(canonicalCWD)

	// When
	mounted, mountErr := rt.ResolveNemoshPath(mountInput)
	tmp, tmpErr := rt.ResolveNemoshPath("/tmp/child.txt")
	device, deviceErr := rt.ResolveNemoshPath("/dev/null")
	_, cygdriveErr := rt.ResolveNemoshPath("/cygdrive/c/child.txt")

	// Then
	if mountErr != nil {
		t.Fatalf("resolve mount alias: %v", mountErr)
	}
	if mounted.Canonical != canonicalCWD {
		t.Fatalf("expected mounted canonical path %q, got %q", canonicalCWD, mounted.Canonical)
	}
	if !sameWindowsPath(mounted.Native, cwd) {
		t.Fatalf("expected mounted native path %q, got %q", cwd, mounted.Native)
	}
	if tmpErr != nil {
		t.Fatalf("resolve tmp path: %v", tmpErr)
	}
	if tmp.Canonical != "/tmp/child.txt" || !sameWindowsPath(tmp.Native, filepath.Join(tmpRoot, "child.txt")) {
		t.Fatalf("unexpected tmp resolution: canonical=%q native=%q", tmp.Canonical, tmp.Native)
	}
	if deviceErr != nil {
		t.Fatalf("resolve device path: %v", deviceErr)
	}
	// `/dev` is the shell's own on Windows and the system's everywhere else, so what a device
	// path resolves to depends on the platform. The two halves are asserted in full by
	// device_platform_windows_test.go and device_platform_other_test.go; here it is enough that
	// the resolution succeeded and named the path, which is what this test is about.
	if device.Canonical != "/dev/null" {
		t.Fatalf("unexpected device resolution: %+v", device)
	}
	if runtimeProvidesDev && (device.Native != "" || !device.Device) {
		t.Fatalf("on this platform /dev/null should be shell-provided: %+v", device)
	}
	if !runtimeProvidesDev && (device.Native == "" || device.Device) {
		t.Fatalf("on this platform /dev/null should be the system's: %+v", device)
	}
	if !errors.Is(cygdriveErr, pathmodel.ErrCygdriveDisabled) {
		t.Fatalf("expected disabled cygdrive error, got %v", cygdriveErr)
	}
	if got := rt.WorkingDirectory(); got != string(canonicalCWD) {
		t.Fatalf("expected canonical working directory %q, got %q", canonicalCWD, got)
	}
	if got := rt.ResolvePath(mountInput); !sameWindowsPath(got, cwd) {
		t.Fatalf("expected compatibility resolver path %q, got %q", cwd, got)
	}
}

func canonicalWindowsPath(t *testing.T, native string) pathmodel.Path {
	t.Helper()
	model := pathmodel.New(pathmodel.DefaultConfig(), "/c")
	canonical, err := model.Resolve(filepath.ToSlash(native))
	if err != nil {
		t.Fatalf("canonicalize Windows fixture %q: %v", native, err)
	}
	return canonical
}

func sameWindowsPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
