package applets_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// dirname drops trailing separators before the last component, as POSIX has it and busybox-w32
// answers: `dirname /a/b/` is /a, where it printed /a/b. Either slash is a separator and stays
// as written, and a drive keeps its root. Each answer was measured against busybox-w32.
func TestDirname_dropsTrailingSeparatorsFirst(t *testing.T) {
	dirname, ok := applets.DefaultRegistry.Lookup("dirname")
	if !ok {
		t.Fatal("dirname is not registered")
	}
	for operand, want := range map[string]string{
		"/a/b/": "/a", "/a/b//": "/a", "a/": ".", "a/b///": "a", "a/b/c/": "a/b",
		"/": "/", "a": ".", "a/b": "a", "/a": "/", "a//b": "a", "": ".", "./a": ".", "../": ".",
		"C:/x/": "C:/", "c:/": "c:/", "C:": "C:.", "C:x": "C:.", "C:/x/y": "C:/x", "C:x/y": "C:x",
		`a\b`: "a", `a\b\`: "a", `C:\x\y\`: `C:\x`,
	} {
		var stdout bytes.Buffer
		if err := dirname.Run(context.Background(), []string{operand}, &bytes.Buffer{}, &stdout, &bytes.Buffer{}); err != nil {
			t.Fatalf("dirname %q: %v", operand, err)
		}
		if got := stdout.String(); got != want+"\n" {
			t.Errorf("dirname %q printed %q, want %q", operand, got, want+"\n")
		}
	}
}
