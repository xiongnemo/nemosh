package applets_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_printsMatchingLines_whenGrepRunsFromStdin(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"two"}, strings.NewReader("one\ntwo\nthree\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected grep to succeed, got %v", err)
	}
	if got := stdout.String(); got != "two\n" {
		t.Fatalf("expected grep output %q, got %q", "two\n", got)
	}
}

func TestDefaultRegistry_filtersMatchingLines_whenGrepRunsWithInvertMatch(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-v", "two"}, strings.NewReader("one\ntwo\nthree\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected grep to succeed, got %v", err)
	}
	if got := stdout.String(); got != "one\nthree\n" {
		t.Fatalf("expected grep output %q, got %q", "one\nthree\n", got)
	}
}

func TestDefaultRegistry_printsLineNumbers_whenGrepRunsWithDashN(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n", "two"}, strings.NewReader("one\ntwo\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected grep to succeed, got %v", err)
	}
	if got := stdout.String(); got != "2:two\n" {
		t.Fatalf("expected grep output %q, got %q", "2:two\n", got)
	}
}

func TestDefaultRegistry_matchesCaseInsensitively_whenGrepRunsWithDashI(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-i", "two"}, strings.NewReader("TWO\nthree\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected grep to succeed, got %v", err)
	}
	if got := stdout.String(); got != "TWO\n" {
		t.Fatalf("expected grep output %q, got %q", "TWO\n", got)
	}
}

func TestDefaultRegistry_returnsFalse_whenGrepFindsNoMatch(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), []string{"missing"}, strings.NewReader("one\n"), &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
}

func TestDefaultRegistry_readsFiles_whenGrepRunsWithPath(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("grep")
	if !ok {
		t.Fatal("expected grep applet to be registered")
	}
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"two", path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected grep to succeed, got %v", err)
	}
	if got := stdout.String(); got != "two\n" {
		t.Fatalf("expected grep output %q, got %q", "two\n", got)
	}
}
