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

// ls, stat, du, mkdir and rmdir go on past an operand they cannot use, as busybox's do. Each
// names it on stderr, does the rest, and exits 1. Each stopped at that operand and did nothing
// for the others. du also printed `0 missing.txt` for one that was not there, and exited 0.
func TestFileApplets_goOnPastAnOperandTheyCannotUse(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
		want   string
		// check looks at what the command left behind, when it changes the tree.
		check func(t *testing.T, dir string)
	}{
		{applet: "ls", args: []string{"missing.txt", "ok.txt"}, want: "ok.txt\n"},
		{applet: "stat", args: []string{"-c", "%n", "missing.txt", "ok.txt"}, want: "ok.txt\n"},
		{applet: "du", args: []string{"-s", "missing.txt", "ok.txt"}, want: "4\tok.txt\n"},
		{applet: "mkdir", args: []string{"nodir/a", "made"}, check: func(t *testing.T, dir string) {
			if info, err := os.Stat(filepath.Join(dir, "made")); err != nil || !info.IsDir() {
				t.Fatalf("made after the failure: %v, want a directory", err)
			}
		}},
		{applet: "rmdir", args: []string{"missingdir", "empty"}, check: func(t *testing.T, dir string) {
			if _, err := os.Stat(filepath.Join(dir, "empty")); !os.IsNotExist(err) {
				t.Fatalf("empty after the failure: %v, want it removed", err)
			}
		}},
	} {
		t.Run(test.applet, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("hi there\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "empty"), 0o755); err != nil {
				t.Fatal(err)
			}
			applet, ok := applets.DefaultRegistry.Lookup(test.applet)
			if !ok {
				t.Fatalf("%s is not registered", test.applet)
			}
			ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
			var stdout, stderr bytes.Buffer
			err := applet.Run(ctx, test.args, &bytes.Buffer{}, &stdout, &stderr)
			if code, isStatus := applets.StatusCode(err); !isStatus || code != 1 {
				t.Fatalf("returned %v, want status 1", err)
			}
			if stdout.String() != test.want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), test.want)
			}
			if !strings.HasPrefix(stderr.String(), test.applet+": ") {
				t.Fatalf("stderr = %q, want the failing operand named under %s", stderr.String(), test.applet)
			}
			if test.check != nil {
				test.check(t, dir)
			}
		})
	}
}
