package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUniqApplet_returnsName_whenConstructed(t *testing.T) {
	// Given
	applet := newUniqApplet()

	// When
	got := applet.Name()

	// Then
	if got != "uniq" {
		t.Fatalf("expected uniq applet name, got %q", got)
	}
}

func TestUniqApplet_collapsesAdjacentDuplicates_whenRunWithStdin(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, bytes.NewBufferString("a\na\nb\nb\na\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\na\n"; got != want {
		t.Fatalf("expected adjacent duplicates collapsed stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_preservesNonAdjacentDuplicates_whenRunWithStdin(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, bytes.NewBufferString("a\nb\na\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\na\n"; got != want {
		t.Fatalf("expected non-adjacent duplicates preserved stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_readsStdin_whenRunWithDashOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-"}, bytes.NewBufferString("a\na\nb\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq - to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected stdin stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_readsStdin_whenRunWithOptionTerminator(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"--"}, bytes.NewBufferString("a\na\nb\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq -- to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected option-terminated stdin stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_collapsesAdjacentDuplicates_whenRunWithFileOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	path := writeUniqFixture(t, "a\na\nb\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq file operand to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected file stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_collapsesAdjacentDuplicates_whenRunWithOptionTerminatedFileOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	path := writeUniqFixture(t, "a\na\nb\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"--", path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq -- file operand to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected option-terminated file stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_normalizesCRLF_whenRunWithWindowsLineEndings(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, bytes.NewBufferString("a\r\na\r\nb\r\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uniq to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected CRLF-normalized stdout %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsEmptyOutput_whenRunWithEmptyStdin(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected empty stdin to succeed, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestUniqApplet_returnsStatusOneAndDiagnostic_whenRunWithInvalidOption(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-x"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "uniq: invalid option -- x\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsInvalidOptionBeforeOperandCount_whenRunWithInvalidOptionAndFile(t *testing.T) {
	// Given
	applet := newUniqApplet()
	path := writeUniqFixture(t, "a\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-x", path}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "uniq: invalid option -- x\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsStatusOneAndDiagnostic_whenRunWithMissingFile(t *testing.T) {
	// Given
	applet := newUniqApplet()
	path := filepath.Join(t.TempDir(), "missing.txt")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	want := "uniq: cannot open '" + path + "': No such file or directory\n"
	if got := stderr.String(); got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsStatusOneAndDiagnostic_whenRunWithEmptyFileOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{""}, bytes.NewBufferString("stdin\n"), &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "uniq: cannot open '': No such file or directory\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsStatusOneAndDiagnostic_whenRunWithOptionTerminatedEmptyFileOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"--", ""}, bytes.NewBufferString("stdin\n"), &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "uniq: cannot open '': No such file or directory\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestUniqApplet_returnsStatusOneAndDiagnostic_whenRunWithExtraOperand(t *testing.T) {
	// Given
	applet := newUniqApplet()
	first := writeUniqFixture(t, "a\n")
	second := writeUniqFixture(t, "b\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{first, second}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertUniqStatus(t, err, 1)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "uniq: too many operands\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func writeUniqFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	return path
}

func assertUniqStatus(t *testing.T, err error, want int) {
	t.Helper()
	got, ok := StatusCode(err)
	if !ok || got != want {
		t.Fatalf("expected applet status %d, got status=%d ok=%v err=%v", want, got, ok, err)
	}
}
