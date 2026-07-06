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

func TestDefaultRegistry_printsWorkingDirectory_whenPwdRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("pwd")
	if !ok {
		t.Fatal("expected pwd applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected pwd to succeed, got %v", err)
	}
	if !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("expected pwd output to end with newline, got %q", stdout.String())
	}
}

func TestDefaultRegistry_printsFirstTenLines_whenHeadRunsWithFile(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("head")
	if !ok {
		t.Fatal("expected head applet to be registered")
	}
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected head to succeed, got %v", err)
	}
	if got := stdout.String(); got != "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n" {
		t.Fatalf("expected head output first ten lines, got %q", got)
	}
}

func TestDefaultRegistry_countsLinesWordsBytes_whenWcRunsFromStdin(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("wc")
	if !ok {
		t.Fatal("expected wc applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader("one two\nthree\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected wc to succeed, got %v", err)
	}
	if got := stdout.String(); got != "2 3 14\n" {
		t.Fatalf("expected wc output %q, got %q", "2 3 14\n", got)
	}
}

func TestDefaultRegistry_printsRequestedLines_whenHeadRunsWithDashN(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("head")
	if !ok {
		t.Fatal("expected head applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n", "2"}, strings.NewReader("a\nb\nc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected head to succeed, got %v", err)
	}
	if got := stdout.String(); got != "a\nb\n" {
		t.Fatalf("expected head output %q, got %q", "a\nb\n", got)
	}
}

func TestDefaultRegistry_printsLastTenLines_whenTailRunsWithFile(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("tail")
	if !ok {
		t.Fatal("expected tail applet to be registered")
	}
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected tail to succeed, got %v", err)
	}
	if got := stdout.String(); got != "2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n" {
		t.Fatalf("expected tail output last ten lines, got %q", got)
	}
}

func TestDefaultRegistry_printsRequestedLines_whenTailRunsWithDashN(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("tail")
	if !ok {
		t.Fatal("expected tail applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n", "2"}, strings.NewReader("a\nb\nc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected tail to succeed, got %v", err)
	}
	if got := stdout.String(); got != "b\nc\n" {
		t.Fatalf("expected tail output %q, got %q", "b\nc\n", got)
	}
}

func TestDefaultRegistry_countsSelectedMetric_whenWcRunsWithFlags(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("wc")
	if !ok {
		t.Fatal("expected wc applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-l", "-w", "-c"}, strings.NewReader("one two\nthree\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected wc to succeed, got %v", err)
	}
	if got := stdout.String(); got != "2 3 14\n" {
		t.Fatalf("expected wc output %q, got %q", "2 3 14\n", got)
	}
}
