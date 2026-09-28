package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// wc pads its counts to nine columns unless it has one count for at most one file, as
// busybox-w32's does: `wc -l a b` aligns its lines and its total. They came out unpadded.
func TestWc_padsOneCountForSeveralFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\ny\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wc, ok := applets.DefaultRegistry.Lookup("wc")
	if !ok {
		t.Fatal("wc is not registered")
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-l", "a.txt", "b.txt"}, want: "        2 a.txt\n        2 b.txt\n        4 total\n"},
		{args: []string{"-l", "a.txt"}, want: "2 a.txt\n"},
	} {
		var stdout bytes.Buffer
		if err := wc.Run(ctx, test.args, &bytes.Buffer{}, &stdout, &bytes.Buffer{}); err != nil {
			t.Fatalf("wc %v: %v", test.args, err)
		}
		if stdout.String() != test.want {
			t.Fatalf("wc %v printed %q, want %q", test.args, stdout.String(), test.want)
		}
	}
}
