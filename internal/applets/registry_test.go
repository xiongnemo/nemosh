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

func TestDefaultRegistry_runsTrue_whenLookupByName(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("true")
	if !ok {
		t.Fatal("expected true applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected true to succeed, got %v", err)
	}
}

func TestDefaultRegistry_copiesCatInput_whenCatRunsWithoutFiles(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("expected cat applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader("data"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cat to succeed, got %v", err)
	}
	if got := stdout.String(); got != "data" {
		t.Fatalf("expected cat output %q, got %q", "data", got)
	}
}

func TestDefaultRegistry_printsEnvironment_whenEnvRuns(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_TEST_ENV", "ok")
	applet, ok := applets.DefaultRegistry.Lookup("printenv")
	if !ok {
		t.Fatal("expected printenv applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"NEMOSH_TEST_ENV"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected printenv to succeed, got %v", err)
	}
	if got := stdout.String(); got != "ok\n" {
		t.Fatalf("expected printenv output %q, got %q", "ok\n", got)
	}
}

func TestDefaultRegistry_evaluatesTestStrings_whenTestRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("test")
	if !ok {
		t.Fatal("expected test applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), []string{"x", "=", "x"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected test to succeed, got %v", err)
	}
}

func TestDefaultRegistry_formatsPrintf_whenPrintfRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("printf")
	if !ok {
		t.Fatal("expected printf applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"%s\\n", "hi"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected printf to succeed, got %v", err)
	}
	if got := stdout.String(); got != "hi\n" {
		t.Fatalf("expected printf output %q, got %q", "hi\n", got)
	}
}

func TestDefaultRegistry_printsBaseName_whenBasenameRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("basename")
	if !ok {
		t.Fatal("expected basename applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"/tmp/file.txt"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected basename to succeed, got %v", err)
	}
	if got := stdout.String(); got != "file.txt\n" {
		t.Fatalf("expected basename output %q, got %q", "file.txt\n", got)
	}
}

func TestDefaultRegistry_printsDirName_whenDirnameRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("dirname")
	if !ok {
		t.Fatal("expected dirname applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"/tmp/file.txt"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected dirname to succeed, got %v", err)
	}
	if got := stdout.String(); got != "/tmp\n" {
		t.Fatalf("expected dirname output %q, got %q", "/tmp\n", got)
	}
}

func TestDefaultRegistry_createsAndRemovesFile_whenTouchAndRmRun(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	touch, ok := applets.DefaultRegistry.Lookup("touch")
	if !ok {
		t.Fatal("expected touch applet to be registered")
	}
	rm, ok := applets.DefaultRegistry.Lookup("rm")
	if !ok {
		t.Fatal("expected rm applet to be registered")
	}

	// When
	touchErr := touch.Run(context.Background(), []string{path}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	rmErr := rm.Run(context.Background(), []string{path}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if touchErr != nil {
		t.Fatalf("expected touch to succeed, got %v", touchErr)
	}
	if rmErr != nil {
		t.Fatalf("expected rm to succeed, got %v", rmErr)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be removed, stat error %v", err)
	}
}

func TestDefaultRegistry_createsAndRemovesDirectory_whenMkdirAndRmdirRun(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "dir")
	mkdir, ok := applets.DefaultRegistry.Lookup("mkdir")
	if !ok {
		t.Fatal("expected mkdir applet to be registered")
	}
	rmdir, ok := applets.DefaultRegistry.Lookup("rmdir")
	if !ok {
		t.Fatal("expected rmdir applet to be registered")
	}

	// When
	mkdirErr := mkdir.Run(context.Background(), []string{path}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	rmdirErr := rmdir.Run(context.Background(), []string{path}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if mkdirErr != nil {
		t.Fatalf("expected mkdir to succeed, got %v", mkdirErr)
	}
	if rmdirErr != nil {
		t.Fatalf("expected rmdir to succeed, got %v", rmdirErr)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected directory to be removed, stat error %v", err)
	}
}

func TestDefaultRegistry_runsFalse_whenLookupByName(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("false")
	if !ok {
		t.Fatal("expected false applet to be registered")
	}

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
}

func TestDefaultRegistry_writesEchoOutput_whenEchoRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("echo")
	if !ok {
		t.Fatal("expected echo applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"hello", "world"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected echo to succeed, got %v", err)
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Fatalf("expected echo output %q, got %q", "hello world\n", got)
	}
}

func TestDefaultRegistry_sortsLines_whenSortRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("sort")
	if !ok {
		t.Fatal("expected sort applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader("c\na\nb\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sort to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\nb\nc\n"; got != want {
		t.Fatalf("expected sort output %q, got %q", want, got)
	}
}
