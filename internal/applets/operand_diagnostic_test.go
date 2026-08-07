package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runApplet(t *testing.T, name string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("expected %s applet to be registered", name)
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), args, &bytes.Buffer{}, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// A required operand that is simply absent used to leave the caller with a bare
// non-zero status and no idea which operand was wanted.
func TestApplets_nameWhatIsMissing_whenARequiredOperandIsAbsent(t *testing.T) {
	for _, name := range []string{"basename", "dirname", "chmod", "cp", "mv", "rm", "touch", "mkdir", "rmdir", "sed"} {
		t.Run(name, func(t *testing.T) {
			// When
			_, _, err := runApplet(t, name)

			// Then
			if err == nil {
				t.Fatal("expected a failure")
			}
			if !strings.Contains(err.Error(), "missing operand") {
				t.Fatalf("err = %v, want it to say what is missing", err)
			}
		})
	}
}

// An unrecognised option used to be taken as an operand, so the applet did
// something other than what it was asked without saying so. `wc -z FILE` was
// the worst of them: it selected no counts at all and still exited 0.
func TestApplets_refuseAnUnknownOption(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("one two\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	for _, testCase := range []struct {
		name string
		args []string
	}{
		{name: "wc", args: []string{"-z", file}},
		{name: "touch", args: []string{"-z", filepath.Join(dir, "made.txt")}},
		{name: "basename", args: []string{"-z", "/a/b"}},
		{name: "mkdir", args: []string{"-z", filepath.Join(dir, "made")}},
		{name: "rmdir", args: []string{"-z", dir}},
		{name: "cp", args: []string{"-z", file, filepath.Join(dir, "b.txt")}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// When
			stdout, _, err := runApplet(t, testCase.name, testCase.args...)

			// Then
			if err == nil {
				t.Fatalf("expected a failure, got stdout %q", stdout)
			}
			if !strings.Contains(err.Error(), "invalid option") || !strings.Contains(err.Error(), "z") {
				t.Fatalf("err = %v, want an invalid-option diagnostic naming z", err)
			}
		})
	}
}

func TestWc_selectsTheDefaultCounts_whenNoFlagIsGiven(t *testing.T) {
	// Given
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("one two\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// When
	stdout, _, err := runApplet(t, "wc", file)

	// Then
	if err != nil {
		t.Fatalf("wc: %v", err)
	}
	if fields := strings.Fields(stdout); len(fields) != 4 {
		t.Fatalf("stdout = %q, want three counts and the name", stdout)
	}
}

func TestChmod_reportsABadModeMessageFirst(t *testing.T) {
	// busybox is message-first and quotes the operand (coreutils/chmod.c:87);
	// this used to be operand-first and unquoted.
	// Given
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// When
	_, _, err := runApplet(t, "chmod", "zzz", file)

	// Then
	if err == nil || err.Error() != "invalid mode 'zzz'" {
		t.Fatalf("err = %v, want %q", err, "invalid mode 'zzz'")
	}
}

func TestRm_removesATreeWithTheRecursiveFlag(t *testing.T) {
	// Given
	dir := t.TempDir()
	branch := filepath.Join(dir, "branch")
	if err := os.MkdirAll(filepath.Join(branch, "inner"), 0o777); err != nil {
		t.Fatalf("seed tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(branch, "inner", "leaf.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed leaf: %v", err)
	}

	// When
	_, _, err := runApplet(t, "rm", "-r", branch)

	// Then
	if err != nil {
		t.Fatalf("rm -r: %v", err)
	}
	if _, statErr := os.Stat(branch); !os.IsNotExist(statErr) {
		t.Fatalf("expected the tree to be gone, stat gave %v", statErr)
	}
}

func TestRm_staysQuietAboutAMissingOperand_whenForced(t *testing.T) {
	// `rm -f build.out` in a cleanup script must not fail because the file was
	// already gone.
	// When
	_, _, err := runApplet(t, "rm", "-f", filepath.Join(t.TempDir(), "absent.txt"))

	// Then
	if err != nil {
		t.Fatalf("rm -f on a missing file: %v", err)
	}
}

func TestRm_reportsAMissingOperand_whenNotForced(t *testing.T) {
	// When
	_, _, err := runApplet(t, "rm", filepath.Join(t.TempDir(), "absent.txt"))

	// Then
	if err == nil {
		t.Fatal("expected rm without -f to report a missing file")
	}
}
