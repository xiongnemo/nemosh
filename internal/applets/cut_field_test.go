package applets

import (
	"bytes"
	"context"
	"testing"
)

func TestCutApplet_selectsFields_whenRunWithDefaultTabDelimiter(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-f", "2"}, bytes.NewBufferString("a\tb\tc\nx\ty\tz\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -f to succeed, got %v", err)
	}
	if got, want := stdout.String(), "b\ny\n"; got != want {
		t.Fatalf("expected default-delimiter field stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsFieldRange_whenRunWithDashF(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-f", "2-"}, bytes.NewBufferString("a\tb\tc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut field range to succeed, got %v", err)
	}
	if got, want := stdout.String(), "b\tc\n"; got != want {
		t.Fatalf("expected field range stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsFields_whenRunWithCustomDelimiter(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-d", ":", "-f", "1,3"}, bytes.NewBufferString("a:b:c\n1:2:3\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut custom delimiter to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a:c\n1:3\n"; got != want {
		t.Fatalf("expected custom-delimiter stdout %q, got %q", want, got)
	}
}

func TestCutApplet_passesLinesWithoutDelimiter_whenSuppressNotSet(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-d", ":", "-f", "2"}, bytes.NewBufferString("alpha\nleft:right\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut without -s to succeed, got %v", err)
	}
	if got, want := stdout.String(), "alpha\nright\n"; got != want {
		t.Fatalf("expected unsuppressed stdout %q, got %q", want, got)
	}
}

func TestCutApplet_suppressesLinesWithoutDelimiter_whenRunWithDashS(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-s", "-d", ":", "-f", "2"}, bytes.NewBufferString("alpha\nleft:right\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -s to succeed, got %v", err)
	}
	if got, want := stdout.String(), "right\n"; got != want {
		t.Fatalf("expected suppressed stdout %q, got %q", want, got)
	}
}

func TestCutApplet_preservesEmptySelectedField_whenInputHasAdjacentDelimiters(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-d", ":", "-f", "2"}, bytes.NewBufferString("a::c\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut empty selected field to succeed, got %v", err)
	}
	if got, want := stdout.String(), "\n"; got != want {
		t.Fatalf("expected empty selected field stdout %q, got %q", want, got)
	}
}

func TestCutApplet_preservesEmptyFieldSeparator_whenRangeIncludesEmptyField(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-d", ":", "-f", "1-2"}, bytes.NewBufferString("a::c\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut empty field range to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a:\n"; got != want {
		t.Fatalf("expected empty field range stdout %q, got %q", want, got)
	}
}

func TestCutApplet_returnsStatusTwoAndDiagnostic_whenRunWithDelimiterWithoutFieldMode(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-d", ":", "-b", "1"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertCutStatus(t, err, 2)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "cut: -d DELIM requires -f\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestCutApplet_returnsStatusTwoAndDiagnostic_whenRunWithSuppressWithoutFieldMode(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-s", "-c", "1"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertCutStatus(t, err, 2)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "cut: -s requires -f\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}

func TestCutApplet_returnsStatusTwoAndDiagnostic_whenRunWithEmptyDelimiter(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-f", "1", "-d", ""}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	assertCutStatus(t, err, 2)
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got, want := stderr.String(), "cut: empty delimiter\n"; got != want {
		t.Fatalf("expected stderr %q, got %q", want, got)
	}
}
