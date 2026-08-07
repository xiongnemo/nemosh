package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// nemoshDriveSpelling rewrites C:\a\b as /c/a/b, the form `pwd` prints and the
// form docs/design/windows-path-model.md lists first among the accepted ones.
func nemoshDriveSpelling(t *testing.T, native string) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		return filepath.ToSlash(native)
	}
	volume := filepath.VolumeName(native)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("temporary directory %q is not on a drive letter", native)
	}
	rest := filepath.ToSlash(strings.TrimPrefix(native, volume))
	return "/" + strings.ToLower(volume[:1]) + rest
}

func TestRun_acceptsAScriptOperandInEveryPathFormTheShellPrints(t *testing.T) {
	// The operand reached os.ReadFile unconverted, so `nemosh /c/dir/s.sh`
	// failed with 127 even though `pwd` inside that same shell prints exactly
	// that spelling.
	// Given
	dir := writeScript(t, "hello.sh", "echo ran\n")
	native := filepath.Join(dir, "hello.sh")

	forms := map[string]string{
		"native":        native,
		"forward slash": filepath.ToSlash(native),
		"drive alias":   nemoshDriveSpelling(t, native),
	}
	for name, operand := range forms {
		t.Run(name, func(t *testing.T) {
			// When
			got := runArgs(t, "", "nemosh", operand)

			// Then
			if got.err != nil {
				t.Fatalf("run %q: %v (stderr = %q)", operand, got.err, got.stderr)
			}
			if got.stdout != "ran\n" {
				t.Fatalf("run %q: stdout = %q, want %q", operand, got.stdout, "ran\n")
			}
		})
	}
}

func TestRun_namesTheOperandAsWritten_whenAScriptCannotBeOpened(t *testing.T) {
	// Given
	dir := writeScript(t, "present.sh", "echo ran\n")

	// When
	got := runArgs(t, dir, "nemosh", "absent.sh")

	// Then
	if got.status != 127 {
		t.Fatalf("status = %d, want 127", got.status)
	}
	if !strings.Contains(got.stderr, "absent.sh") {
		t.Fatalf("stderr = %q, want it to name the operand", got.stderr)
	}
}
