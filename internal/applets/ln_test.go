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

// ln takes busybox-w32's forms and options (coreutils/ln.c), each measured against it. It took
// -s alone and exactly two operands, so `ln -f a b`, `ln a b dir` and `ln dir/file` failed, and
// `ln -sf target link`, the way a script replaces a link, failed on its option.
func TestLn_takesBusyboxFormsAndOptions(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"a": "A\n", "b": "B\n", "sub/s": "S\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	run := func(args ...string) (string, string, error) {
		var stdout, stderr bytes.Buffer
		err := lookupLn(t).Run(ctx, args, &bytes.Buffer{}, &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}
	same := func(left, right string) bool {
		leftInfo, leftErr := os.Stat(filepath.Join(dir, left))
		rightInfo, rightErr := os.Stat(filepath.Join(dir, right))
		return leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo)
	}

	if _, _, err := run("a", "b"); err == nil || !strings.Contains(err.Error(), "b: File exists") {
		t.Fatalf("ln a b over an existing b returned %v, want b: File exists", err)
	}
	if _, _, err := run("-f", "a", "b"); err != nil || !same("a", "b") {
		t.Fatalf("ln -f a b returned %v, and b is a link to a: %v", err, same("a", "b"))
	}
	if stdout, _, err := run("-v", "a", "c"); err != nil || stdout != "'c' -> 'a'\n" || !same("a", "c") {
		t.Fatalf("ln -v a c printed %q and returned %v", stdout, err)
	}
	if _, _, err := run("-b", "b", "c"); err != nil || !same("b", "c") || !same("a", "c~") {
		t.Fatalf("ln -b b c returned %v; c is b: %v, c~ is the old c: %v", err, same("b", "c"), same("a", "c~"))
	}
	if _, _, err := run("a", "b", "d"); err != nil || !same("a", "d/a") || !same("b", "d/b") {
		t.Fatalf("ln a b d returned %v, want d/a and d/b", err)
	}
	if _, _, err := run("sub/s"); err != nil || !same("sub/s", "s") {
		t.Fatalf("ln sub/s returned %v, want s in the working directory", err)
	}
	if _, _, err := run("-T", "a", "d"); err == nil || err.Error() != "'d' is a directory" {
		t.Fatalf("ln -T a d returned %v", err)
	}
	if _, _, err := run("-T", "a", "b", "c"); err == nil || err.Error() != "-T accepts 2 args max" {
		t.Fatalf("ln -T a b c returned %v", err)
	}
	if _, _, err := run(); err == nil {
		t.Fatal("ln with no operands succeeded")
	}
	if _, _, err := run("-z", "a", "b"); err == nil || !strings.Contains(err.Error(), "invalid option") {
		t.Fatalf("ln -z returned %v", err)
	}
	// A target that cannot be linked is named, and the rest are still linked, status 1.
	_, stderr, err := run("missing", "sub/s", "d")
	if code, isStatus := applets.StatusCode(err); !isStatus || code != 1 || !same("sub/s", "d/s") ||
		!strings.Contains(stderr, "missing: No such file or directory") {
		t.Fatalf("ln missing sub/s d returned %v, stderr %q; d/s linked: %v", err, stderr, same("sub/s", "d/s"))
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
