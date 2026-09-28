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

// cmp and diff answer 2 for an operand they cannot read, as busybox's do, and keep 1 for files
// that differ. Both answered 1 for a missing operand, so a script could not tell trouble from a
// difference.
func TestCompare_aMissingOperandIsTroubleNotADifference(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"a.txt": "a\n", "b.txt": "b\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	for _, test := range []struct {
		applet string
		args   []string
		status int
	}{
		{applet: "cmp", args: []string{"missing.txt", "a.txt"}, status: 2},
		{applet: "diff", args: []string{"missing.txt", "a.txt"}, status: 2},
		{applet: "cmp", args: []string{"a.txt", "b.txt"}, status: 1},
		{applet: "diff", args: []string{"a.txt", "b.txt"}, status: 1},
	} {
		applet, ok := applets.DefaultRegistry.Lookup(test.applet)
		if !ok {
			t.Fatalf("%s is not registered", test.applet)
		}
		err := applet.Run(ctx, test.args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
		code, isStatus := applets.StatusCode(err)
		if errors.Is(err, applets.ErrExitFalse) {
			code, isStatus = 1, true
		}
		if !isStatus || code != test.status {
			t.Fatalf("%s %s returned %v, want status %d", test.applet, strings.Join(test.args, " "), err, test.status)
		}
	}
}
