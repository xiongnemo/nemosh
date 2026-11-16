package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chmod reads a file's first bytes only to say its mode, under -v or -c, and writes its
// attributes only when the read-only bit changes: `chmod -R 644` over a tree reads no file, a
// second `chmod -R 444` writes nothing, and the first writes the files and not the directories,
// which are never read-only.
func TestChmod_readsAndWritesOnlyWhatItMust(t *testing.T) {
	savedExecutable, savedWrite := chmodExecutable, chmodWrite
	t.Cleanup(func() { chmodExecutable, chmodWrite = savedExecutable, savedWrite })
	reads, writes := 0, 0
	chmodExecutable = func(native string, info os.FileInfo) bool {
		reads++
		return savedExecutable(native, info)
	}
	chmodWrite = func(native string, mode uint32, isDir bool) error {
		writes++
		return savedWrite(native, mode, isDir)
	}
	dir := t.TempDir()
	for _, name := range []string{"tree/a.txt", "tree/b", "tree/sub/c.txt"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("text\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(dir, func(path string, _ os.DirEntry, _ error) error { return os.Chmod(path, 0o666) })
	})
	applet, _ := DefaultRegistry.Lookup("chmod")
	chmod := func(args ...string) string {
		t.Helper()
		reads, writes = 0, 0
		var stdout, stderr bytes.Buffer
		ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir, environ: os.Environ()})
		if err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("chmod %q: %v, %q", args, err, stderr.String())
		}
		return stdout.String()
	}
	for _, test := range []struct {
		args          []string
		reads, writes int
	}{
		{args: []string{"-R", "644", "tree"}},
		{args: []string{"-R", "444", "tree"}, writes: 3},
		{args: []string{"-R", "444", "tree"}},
		{args: []string{"-R", "a+X", "tree"}},
	} {
		chmod(test.args...)
		if reads != test.reads || writes != test.writes {
			t.Errorf("chmod %q read %d files and wrote %d; want %d and %d", test.args, reads, writes, test.reads, test.writes)
		}
	}
	for _, name := range []string{"a.txt", "b", "sub/c.txt"} {
		if info, err := os.Stat(filepath.Join(dir, "tree", filepath.FromSlash(name))); err != nil || info.Mode().Perm()&0o200 != 0 {
			t.Errorf("%s is %v, %v; want read-only", name, info.Mode(), err)
		}
	}
	// -v says the mode, so it judges the file: the one file named, read once.
	if said := chmod("-v", "644", "tree/b"); reads != 1 || writes != 1 || !strings.Contains(said, "changed to 0644") {
		t.Errorf("chmod -v read %d and wrote %d, and said %q", reads, writes, said)
	}
}
