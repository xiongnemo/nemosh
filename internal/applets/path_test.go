package applets_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_registersPathApplets(t *testing.T) {
	// Given
	names := []string{"winpath", "posixpath"}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			// When
			_, ok := applets.DefaultRegistry.Lookup(name)

			// Then
			if !ok {
				t.Fatalf("expected %s applet to be registered", name)
			}
		})
	}
}

func TestDefaultRegistry_printsWindowsPath_whenWinpathRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("winpath")
	if !ok {
		t.Fatal("expected winpath applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"/c/Users/nemo"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected winpath to succeed, got %v", err)
	}
	if got := stdout.String(); got != "C:/Users/nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "C:/Users/nemo\n", got)
	}
}

func TestDefaultRegistry_printsUNCPath_whenWinpathRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("winpath")
	if !ok {
		t.Fatal("expected winpath applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"//server/share/dir"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected winpath to succeed, got %v", err)
	}
	if got := stdout.String(); got != "//server/share/dir\n" {
		t.Fatalf("expected stdout %q, got %q", "//server/share/dir\n", got)
	}
}

func TestDefaultRegistry_printsUNCPath_whenPosixpathRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("posixpath")
	if !ok {
		t.Fatal("expected posixpath applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"//server/share/dir"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected posixpath to succeed, got %v", err)
	}
	if got := stdout.String(); got != "//server/share/dir\n" {
		t.Fatalf("expected stdout %q, got %q", "//server/share/dir\n", got)
	}
}

func TestDefaultRegistry_returnsNoWindowsPath_whenWinpathRunsWithVirtualRoot(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("winpath")
	if !ok {
		t.Fatal("expected winpath applet to be registered")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"/tmp/file"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "path has no Windows spelling\n" {
		t.Fatalf("expected stderr %q, got %q", "path has no Windows spelling\n", got)
	}
}

func TestDefaultRegistry_printsPosixPath_whenPosixpathRunsWithWindowsPath(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("posixpath")
	if !ok {
		t.Fatal("expected posixpath applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"C:/Users/nemo"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected posixpath to succeed, got %v", err)
	}
	if got := stdout.String(); got != "/c/Users/nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "/c/Users/nemo\n", got)
	}
}

func TestDefaultRegistry_printsPosixPath_whenPosixpathRunsWithWindowsBackslashPath(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("posixpath")
	if !ok {
		t.Fatal("expected posixpath applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{`C:\Users\nemo`}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected posixpath to succeed, got %v", err)
	}
	if got := stdout.String(); got != "/c/Users/nemo\n" {
		t.Fatalf("expected stdout %q, got %q", "/c/Users/nemo\n", got)
	}
}

func TestDefaultRegistry_returnsHostOnlyUNCError_whenPosixpathRunsWithHostOnlyUNC(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("posixpath")
	if !ok {
		t.Fatal("expected posixpath applet to be registered")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"//server"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "//server is not a directory root; use //server/share\n" {
		t.Fatalf("expected stderr %q, got %q", "//server is not a directory root; use //server/share\n", got)
	}
}

func TestDefaultRegistry_preservesPreviousOutput_whenWinpathErrorsAfterSuccessfulOperand(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("winpath")
	if !ok {
		t.Fatal("expected winpath applet to be registered")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"/c/ok", "/tmp/file"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "C:/ok\n" {
		t.Fatalf("expected stdout %q, got %q", "C:/ok\n", got)
	}
	if got := stderr.String(); got != "path has no Windows spelling\n" {
		t.Fatalf("expected stderr %q, got %q", "path has no Windows spelling\n", got)
	}
}

func TestDefaultRegistry_returnsFalse_whenPathAppletRunsWithoutOperands(t *testing.T) {
	// Given
	names := []string{"winpath", "posixpath"}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			applet, ok := applets.DefaultRegistry.Lookup(name)
			if !ok {
				t.Fatalf("expected %s applet to be registered", name)
			}

			// When
			err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

			// Then
			if !errors.Is(err, applets.ErrExitFalse) {
				t.Fatalf("expected ErrExitFalse, got %v", err)
			}
		})
	}
}
