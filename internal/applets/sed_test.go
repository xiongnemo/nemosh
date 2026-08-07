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

func runSed(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("sed")
	if !ok {
		t.Fatal("expected sed applet to be registered")
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestSed_readsTheFileOperands(t *testing.T) {
	// `sed 's/a/b/' notes.txt` used to exit 1 with no diagnostic: the operand
	// was neither used nor refused.
	// Given
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatalf("seed first: %v", err)
	}
	if err := os.WriteFile(second, []byte("gamma\n"), 0o600); err != nil {
		t.Fatalf("seed second: %v", err)
	}

	// When
	stdout, stderr, err := runSed(t, "", "s/a/A/", first, second)

	// Then
	if err != nil {
		t.Fatalf("sed: %v (stderr %q)", err, stderr)
	}
	if stdout != "Alpha\nbetA\ngAmma\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "Alpha\nbetA\ngAmma\n")
	}
}

func TestSed_stillReadsStdin_whenThereAreNoOperands(t *testing.T) {
	// When
	stdout, _, err := runSed(t, "one\ntwo\n", "s/o/0/")

	// Then
	if err != nil {
		t.Fatalf("sed: %v", err)
	}
	if stdout != "0ne\ntw0\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "0ne\ntw0\n")
	}
}

func TestSed_replacesEveryMatch_whenTheGlobalFlagIsGiven(t *testing.T) {
	// When
	stdout, _, err := runSed(t, "aaa\n", "s/a/b/g")

	// Then
	if err != nil {
		t.Fatalf("sed: %v", err)
	}
	if stdout != "bbb\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "bbb\n")
	}
}

func TestSed_replacesOnlyTheFirst_whenNoFlagIsGiven(t *testing.T) {
	// When
	stdout, _, err := runSed(t, "aaa\n", "s/a/b/")

	// Then
	if err != nil {
		t.Fatalf("sed: %v", err)
	}
	if stdout != "baa\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "baa\n")
	}
}

func TestSed_replacesTheNamedOccurrence_whenANumberIsGiven(t *testing.T) {
	// When
	second, _, err := runSed(t, "aaa\n", "s/a/b/2")
	if err != nil {
		t.Fatalf("sed s/a/b/2: %v", err)
	}
	fromSecond, _, err := runSed(t, "aaa\n", "s/a/b/2g")
	if err != nil {
		t.Fatalf("sed s/a/b/2g: %v", err)
	}

	// Then
	if second != "aba\n" {
		t.Fatalf("s/a/b/2 = %q, want %q", second, "aba\n")
	}
	if fromSecond != "abb\n" {
		t.Fatalf("s/a/b/2g = %q, want %q", fromSecond, "abb\n")
	}
}

func TestSed_warnsAndKeepsGoing_whenOneOperandCannotBeRead(t *testing.T) {
	// Given
	dir := t.TempDir()
	present := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(present, []byte("kept\n"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// When
	stdout, stderr, err := runSed(t, "", "s/k/K/", filepath.Join(dir, "absent.txt"), present)

	// Then
	if err == nil {
		t.Fatal("expected a non-zero status for the unreadable operand")
	}
	if status, ok := applets.StatusCode(err); !ok || status != 1 {
		t.Fatalf("status = %v, want 1", err)
	}
	if stdout != "Kept\n" {
		t.Fatalf("stdout = %q, want the readable operand to still be processed", stdout)
	}
	if !strings.Contains(stderr, "absent.txt") {
		t.Fatalf("stderr = %q, want it to name the unreadable operand", stderr)
	}
}

func TestSed_refusesAnUnknownSubstituteFlag(t *testing.T) {
	// When
	_, _, err := runSed(t, "a\n", "s/a/b/Z")

	// Then
	if err == nil || !strings.Contains(err.Error(), "Z") {
		t.Fatalf("err = %v, want an unknown-flag diagnostic naming Z", err)
	}
}

func TestSed_reportsAMissingScript(t *testing.T) {
	// When
	_, _, err := runSed(t, "a\n")

	// Then
	if err == nil || !strings.Contains(err.Error(), "missing operand") {
		t.Fatalf("err = %v, want a missing-operand diagnostic", err)
	}
}
