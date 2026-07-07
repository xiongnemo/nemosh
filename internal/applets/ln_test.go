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

func TestDefaultRegistry_registersLn_whenLookupByName(t *testing.T) {
	// Given
	name := "ln"

	// When
	_, ok := applets.DefaultRegistry.Lookup(name)

	// Then
	if !ok {
		t.Fatal("expected ln applet to be registered")
	}
}

func TestDefaultRegistry_createsHardLink_whenLnRuns(t *testing.T) {
	// Given
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	linkName := filepath.Join(dir, "linked.txt")
	if err := os.WriteFile(source, []byte("link-me"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet := lookupLn(t)

	// When
	err := applet.Run(context.Background(), []string{source, linkName}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected ln to succeed, got %v", err)
	}
	sourceInfo, statSourceErr := os.Stat(source)
	if statSourceErr != nil {
		t.Fatalf("expected source stat to succeed, got %v", statSourceErr)
	}
	linkInfo, statLinkErr := os.Stat(linkName)
	if statLinkErr != nil {
		t.Fatalf("expected link stat to succeed, got %v", statLinkErr)
	}
	if !os.SameFile(sourceInfo, linkInfo) {
		t.Fatalf("expected %q and %q to be the same file", source, linkName)
	}
}

func TestDefaultRegistry_createsSymlink_whenLnRunsWithSymbolicFlag(t *testing.T) {
	// Given
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	linkName := filepath.Join(dir, "linked.txt")
	if err := os.WriteFile(target, []byte("link-me"), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	applet := lookupLn(t)

	// When
	err := applet.Run(context.Background(), []string{"-s", target, linkName}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		message := strings.ToLower(err.Error())
		if os.IsPermission(err) || strings.Contains(message, "privilege") {
			t.Skipf("skipping symlink assertion because this Windows environment lacks symlink permission: %v", err)
		}
		t.Fatalf("expected ln -s to succeed, got %v", err)
	}
	got, readlinkErr := os.Readlink(linkName)
	if readlinkErr != nil {
		t.Fatalf("expected symlink read to succeed, got %v", readlinkErr)
	}
	if got != target {
		t.Fatalf("expected symlink target %q, got %q", target, got)
	}
}

func TestDefaultRegistry_returnsErrExitFalse_whenLnRunsWithWrongArity(t *testing.T) {
	// Given
	applet := lookupLn(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "no operands", args: nil},
		{name: "one operand", args: []string{"only-source"}},
		{name: "three operands", args: []string{"source", "link", "extra"}},
		{name: "symbolic flag only", args: []string{"-s"}},
		{name: "symbolic one operand", args: []string{"-s", "target"}},
		{name: "symbolic three operands", args: []string{"-s", "target", "link", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

			// Then
			if !errors.Is(err, applets.ErrExitFalse) {
				t.Fatalf("expected ln wrong arity to return ErrExitFalse, got %v", err)
			}
		})
	}
}

func TestDefaultRegistry_returnsUnsupportedOptionError_whenLnRunsWithUnknownFlag(t *testing.T) {
	// Given
	applet := lookupLn(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "force", args: []string{"-f", "source", "dest"}, want: "unsupported ln option: -f"},
		{name: "no dereference", args: []string{"-n", "source", "dest"}, want: "unsupported ln option: -n"},
		{name: "target directory", args: []string{"-T", "source", "dest"}, want: "unsupported ln option: -T"},
		{name: "verbose", args: []string{"-v", "source", "dest"}, want: "unsupported ln option: -v"},
		{name: "long symbolic", args: []string{"--symbolic", "source", "dest"}, want: "unsupported ln option: --symbolic"},
		{name: "unknown", args: []string{"-z", "source", "dest"}, want: "unsupported ln option: -z"},
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

func lookupLn(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("ln")
	if !ok {
		t.Fatal("expected ln applet to be registered")
	}
	return applet
}
