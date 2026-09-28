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

// Given an operand it cannot open and then one it can, each filter names the first on stderr,
// reads the second, and exits 1, as busybox's does. The first operand ended the command, and
// nothing of the second was printed.
func TestFilters_goOnPastAnOperandTheyCannotOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("hi there\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "cat", want: "hi there\n"},
		{applet: "tac", want: "hi there\n"},
		{applet: "rev", want: "ereht ih\n"},
		{applet: "fold", args: []string{"-w", "4"}, want: "hi t\nhere\n"},
		{applet: "expand", want: "hi there\n"},
		{applet: "strings", want: "hi there\n"},
		{applet: "nl", want: "     1\thi there\n"},
	} {
		t.Run(test.applet, func(t *testing.T) {
			applet, ok := applets.DefaultRegistry.Lookup(test.applet)
			if !ok {
				t.Fatalf("%s is not registered", test.applet)
			}
			var stdout, stderr bytes.Buffer
			err := applet.Run(ctx, append(test.args, "missing.txt", "ok.txt"), &bytes.Buffer{}, &stdout, &stderr)
			if code, isStatus := applets.StatusCode(err); !isStatus || code != 1 {
				t.Fatalf("returned %v, want status 1", err)
			}
			if stdout.String() != test.want {
				t.Fatalf("stdout = %q, want %q: the readable operand, read", stdout.String(), test.want)
			}
			if !strings.HasPrefix(stderr.String(), test.applet+": ") || !strings.Contains(stderr.String(), "missing.txt") {
				t.Fatalf("stderr = %q, want the missing operand named under %s", stderr.String(), test.applet)
			}
		})
	}
}
