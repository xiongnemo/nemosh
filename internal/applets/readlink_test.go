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

func TestDefaultRegistry_registersReadlink_whenLookupByName(t *testing.T) {
	// Given
	name := "readlink"

	// When
	_, ok := applets.DefaultRegistry.Lookup(name)

	// Then
	if !ok {
		t.Fatal("expected readlink applet to be registered")
	}
}

func TestDefaultRegistry_printsTargetWithNewline_whenReadlinkRuns(t *testing.T) {
	// Given
	target, linkName := createReadlinkSymlink(t)
	applet := lookupReadlink(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{linkName}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected readlink to succeed, got %v", err)
	}
	if got := stdout.String(); got != target+"\n" {
		t.Fatalf("expected stdout %q, got %q", target+"\n", got)
	}
}

func TestDefaultRegistry_omitsNewline_whenReadlinkRunsWithDashN(t *testing.T) {
	// Given
	target, linkName := createReadlinkSymlink(t)
	applet := lookupReadlink(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n", linkName}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected readlink -n to succeed, got %v", err)
	}
	if got := stdout.String(); got != target {
		t.Fatalf("expected stdout %q, got %q", target, got)
	}
}

func TestDefaultRegistry_returnsErrExitFalse_whenReadlinkRunsWithWrongArity(t *testing.T) {
	// Given
	applet := lookupReadlink(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "no operands", args: nil},
		{name: "dash n only", args: []string{"-n"}},
		{name: "two operands", args: []string{"link", "extra"}},
		{name: "dash n two operands", args: []string{"-n", "link", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

			// Then
			if !errors.Is(err, applets.ErrExitFalse) {
				t.Fatalf("expected readlink wrong arity to return ErrExitFalse, got %v", err)
			}
		})
	}
}

func TestDefaultRegistry_returnsUnsupportedOptionError_whenReadlinkRunsWithUnknownFlag(t *testing.T) {
	// Given
	applet := lookupReadlink(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "canonicalize", args: []string{"-f", "link"}, want: "unsupported readlink option: -f"},
		{name: "verbose", args: []string{"-v", "link"}, want: "unsupported readlink option: -v"},
		{name: "quiet", args: []string{"-q", "link"}, want: "unsupported readlink option: -q"},
		{name: "silent", args: []string{"-s", "link"}, want: "unsupported readlink option: -s"},
		{name: "unknown", args: []string{"-z", "link"}, want: "unsupported readlink option: -z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

			// Then
			if err == nil || err.Error() != tt.want {
				t.Fatalf("expected unsupported option error %q, got %v", tt.want, err)
			}
		})
	}
}

func TestDefaultRegistry_returnsErrExitFalseAndNoOutput_whenReadlinkRunsOnRegularFile(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(path, []byte("not-a-link"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet := lookupReadlink(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected readlink on a regular file to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func TestDefaultRegistry_returnsErrExitFalseAndNoOutput_whenReadlinkRunsOnMissingPath(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "missing.txt")
	applet := lookupReadlink(t)
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected readlink on a missing path to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
}

func createReadlinkSymlink(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	linkName := filepath.Join(dir, "linked.txt")
	if err := os.WriteFile(target, []byte("read-me"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	if err := os.Symlink(target, linkName); err != nil {
		message := strings.ToLower(err.Error())
		if os.IsPermission(err) || strings.Contains(message, "privilege") {
			t.Skipf("skipping symlink assertion because this Windows environment lacks symlink permission: %v", err)
		}
		t.Fatalf("expected symlink fixture creation to succeed, got %v", err)
	}
	return target, linkName
}

func lookupReadlink(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("readlink")
	if !ok {
		t.Fatal("expected readlink applet to be registered")
	}
	return applet
}
