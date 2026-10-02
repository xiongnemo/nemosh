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

func TestDefaultRegistry_refusesAnUnknownOption_whenReadlinkRunsWithOne(t *testing.T) {
	// When
	err := lookupReadlink(t).Run(context.Background(), []string{"-z", "link"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err == nil || err.Error() != "unknown option -- z" {
		t.Fatalf("expected readlink -z to be refused, got %v", err)
	}
}

// readlink -f prints the canonical path realpath prints, and the last component need not be
// there so long as its directory is, as busybox's has it. It was refused, which ended a script
// finding its own directory with `readlink -f "$0"`. -v says why a link could not be read,
// -s and -q are the quiet that is already the default, and the options may follow FILE.
func TestReadlink_canonicalizesAPathWithDashF(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	realpath, ok := applets.DefaultRegistry.Lookup("realpath")
	if !ok {
		t.Fatal("expected realpath applet to be registered")
	}
	var wantFile bytes.Buffer
	if err := realpath.Run(context.Background(), []string{file}, &bytes.Buffer{}, &wantFile, &bytes.Buffer{}); err != nil {
		t.Fatalf("realpath %s: %v", file, err)
	}
	missing := filepath.Join(dir, "missing.txt")
	wantMissing := strings.TrimSuffix(wantFile.String(), "file.txt\n") + "missing.txt\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-f", file}, wantFile.String()},
		{[]string{file, "-f"}, wantFile.String()},
		{[]string{"-fn", file}, strings.TrimSuffix(wantFile.String(), "\n")},
		{[]string{"-fsq", missing}, wantMissing},
	} {
		var stdout bytes.Buffer
		err := lookupReadlink(t).Run(context.Background(), test.args, &bytes.Buffer{}, &stdout, &bytes.Buffer{})
		if err != nil || stdout.String() != test.want {
			t.Errorf("readlink %q: got %q, %v; want %q", test.args, stdout.String(), err, test.want)
		}
	}
	var stdout, stderr bytes.Buffer
	err := lookupReadlink(t).Run(context.Background(), []string{"-f", filepath.Join(dir, "no", "such")}, &bytes.Buffer{}, &stdout, &stderr)
	if !errors.Is(err, applets.ErrExitFalse) || stdout.String() != "" || stderr.String() != "" {
		t.Errorf("readlink -f below a missing directory: got %q, %q, %v; want a quiet status 1", stdout.String(), stderr.String(), err)
	}
	err = lookupReadlink(t).Run(context.Background(), []string{"-v", file}, &bytes.Buffer{}, &stdout, &stderr)
	if want := "readlink: " + file + ": cannot read link: not a symlink\n"; !errors.Is(err, applets.ErrExitFalse) || stderr.String() != want {
		t.Errorf("readlink -v on a file: got %q, %v; want %q", stderr.String(), err, want)
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
