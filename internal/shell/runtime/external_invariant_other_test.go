//go:build !windows

package runtime

import (
	"errors"
	"os"
	"testing"
)

func TestExternalCommandPath_preservesRelativeNativePathError(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("probe", []byte("probe"), 0o700); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	paths := newPathState(".", DefaultPathSettings())
	runtime := Runtime{paths: &paths, vars: map[string]string{"PATH": "."}}

	_, err := runtime.externalCommandPath("probe")

	if !errors.Is(err, errExternalPathNotAbsolute) {
		t.Fatalf("lookup error: got %v, want %v", err, errExternalPathNotAbsolute)
	}
}

func TestHasPathSeparator_doesNotTreatBackslashAsSeparator(t *testing.T) {
	if hasPathSeparator(`tool\name`) {
		t.Fatal(`hasPathSeparator("tool\\name"): got true, want false on non-Windows`)
	}
}
