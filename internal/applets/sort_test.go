package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSortApplet_returnsName_whenConstructed(t *testing.T) {
	// Given
	applet := newSortApplet()

	// When
	got := applet.Name()

	// Then
	if got != "sort" {
		t.Fatalf("expected sort applet name, got %q", got)
	}
}

func TestSortApplet_sortsLines_whenRunWithStdin(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, bytes.NewBufferString("c\na\nb\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\nc\n"; got != want {
		t.Fatalf("expected sorted stdout %q, got %q", want, got)
	}
}

func TestSortApplet_sortsLines_whenRunWithFileOperand(t *testing.T) {
	// Given
	applet := newSortApplet()
	path := writeSortFixture(t, "c\na\nb\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\nc\n"; got != want {
		t.Fatalf("expected sorted file stdout %q, got %q", want, got)
	}
}

func TestSortApplet_sortsCombinedLines_whenRunWithMultipleFiles(t *testing.T) {
	// Given
	applet := newSortApplet()
	first := writeSortFixture(t, "d\na\n")
	second := writeSortFixture(t, "c\nb\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{first, second}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\nc\nd\n"; got != want {
		t.Fatalf("expected combined sorted stdout %q, got %q", want, got)
	}
}

func TestSortApplet_sortsNumericLines_whenRunWithDashN(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n"}, bytes.NewBufferString("10\n2\n1\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort -n to succeed, got %v", err)
	}
	if got, want := stdout.String(), "1\n2\n10\n"; got != want {
		t.Fatalf("expected numeric sorted stdout %q, got %q", want, got)
	}
}

func TestSortApplet_reversesLines_whenRunWithDashR(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-r"}, bytes.NewBufferString("a\nc\nb\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort -r to succeed, got %v", err)
	}
	if got, want := stdout.String(), "c\nb\na\n"; got != want {
		t.Fatalf("expected reverse sorted stdout %q, got %q", want, got)
	}
}

func TestSortApplet_reversesNumericLines_whenRunWithClusteredDashNR(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-nr"}, bytes.NewBufferString("10\n2\n1\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort -nr to succeed, got %v", err)
	}
	if got, want := stdout.String(), "10\n2\n1\n"; got != want {
		t.Fatalf("expected reverse numeric sorted stdout %q, got %q", want, got)
	}
}

func TestSortApplet_stripsCRLF_whenRunWithWindowsLineEndings(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, bytes.NewBufferString("b\r\na\r\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\n"; got != want {
		t.Fatalf("expected CRLF-normalized stdout %q, got %q", want, got)
	}
}

func TestSortApplet_returnsStatusTwoAndDiagnostic_whenRunWithInvalidOption(t *testing.T) {
	// Given
	applet := newSortApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-x"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertSortStatus(t, err, 2)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "sort: invalid option -- x\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestSortApplet_returnsStatusTwoAndDiagnostic_whenRunWithMissingFile(t *testing.T) {
	// Given
	applet := newSortApplet()
	path := filepath.Join(t.TempDir(), "missing.txt")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertSortStatus(t, err, 2)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	want := "sort: " + path + ": No such file or directory\n"
	if got := stderr.String(); got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func writeSortFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	return path
}

func assertSortStatus(t *testing.T, err error, want int) {
	t.Helper()
	got, ok := StatusCode(err)
	if !ok || got != want {
		t.Fatalf("expected applet status %d, got status=%d ok=%v err=%v", want, got, ok, err)
	}
}
