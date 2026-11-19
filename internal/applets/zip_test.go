package applets_test

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// zipTestEntry is what an archive holds of one name, read and the archive closed again: Windows
// keeps a file that is open from being replaced, and zip replaces the archive it updates.
type zipTestEntry struct {
	method                   uint16
	content                  string
	compressed, uncompressed uint64
}

// zip is Info-ZIP's, as Debian has it: `  adding: NAME (deflated N%)` a file, (stored 0%) where
// deflating would not help, -r into directories with an entry for each, -j the names alone, -0 no
// compression, -q nothing said, -D no directory entries, -x's patterns left out, `.zip` added to a
// name without an extension, and an archive there updated, a name in it again `updating:`. The
// percent is this deflate's, so the test holds it to the archive's sizes rather than to zlib's.
// There was no zip.
func TestZip_asInfoZipDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"a.txt": strings.Repeat("abc", 400), "d/b.txt": "b\n", "d/sub/c.o": "obj", "tiny": "x",
	})
	read := func(name string) map[string]zipTestEntry {
		t.Helper()
		reader, err := zip.OpenReader(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer reader.Close()
		entries := map[string]zipTestEntry{}
		for _, file := range reader.File {
			opened, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(opened)
			opened.Close()
			entries[file.Name] = zipTestEntry{method: file.Method, content: string(data), compressed: file.CompressedSize64, uncompressed: file.UncompressedSize64}
		}
		return entries
	}
	names := func(entries map[string]zipTestEntry) string {
		var out []string
		for name := range entries {
			out = append(out, name)
		}
		sort.Strings(out)
		return strings.Join(out, " ")
	}

	stdout, stderr, err := runSmall(t, dir, "", "zip", "out", "a.txt", "tiny")
	if err != nil {
		t.Fatalf("zip out a.txt tiny: %v (%s)", err, stderr)
	}
	entries := read("out.zip")
	a := entries["a.txt"]
	if a.method != zip.Deflate || a.content != strings.Repeat("abc", 400) || entries["tiny"].method != zip.Store {
		t.Fatalf("out.zip holds %+v", entries)
	}
	percent := 100 - int((a.compressed*100+a.uncompressed/2)/a.uncompressed)
	if want := fmt.Sprintf("  adding: a.txt (deflated %d%%)\n  adding: tiny (stored 0%%)\n", percent); stdout != want {
		t.Errorf("zip said %q, want %q", stdout, want)
	}

	if stdout, _, err := runSmall(t, dir, "", "zip", "-r", "tree.zip", "d", "-x", "*.o"); err != nil || stdout != "  adding: d/ (stored 0%)\n  adding: d/b.txt (stored 0%)\n  adding: d/sub/ (stored 0%)\n" {
		t.Errorf("zip -r tree.zip d -x '*.o' = %q, %v", stdout, err)
	}
	if got := names(read("tree.zip")); got != "d/ d/b.txt d/sub/" {
		t.Errorf("tree.zip holds %s", got)
	}
	if stdout, _, err := runSmall(t, dir, "", "zip", "-rqjD0", "flat.zip", "d"); err != nil || stdout != "" {
		t.Errorf("zip -rqjD0 = %q, %v", stdout, err)
	}
	if flat := read("flat.zip"); names(flat) != "b.txt c.o" || flat["b.txt"].method != zip.Store {
		t.Errorf("flat.zip holds %s", names(flat))
	}

	if err := os.WriteFile(filepath.Join(dir, "tiny"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, _, err := runSmall(t, dir, "", "zip", "out.zip", "tiny", "d/b.txt"); err != nil || stdout != "updating: tiny (stored 0%)\n  adding: d/b.txt (stored 0%)\n" {
		t.Errorf("zip into out.zip again = %q, %v", stdout, err)
	}
	if updated := read("out.zip"); updated["tiny"].content != "y" || updated["a.txt"].content != strings.Repeat("abc", 400) || names(updated) != "a.txt d/b.txt tiny" {
		t.Errorf("out.zip after the update holds %s", names(updated))
	}

	_, stderr, err = runSmall(t, dir, "", "zip", "none.zip", "nope")
	if status, _ := applets.StatusCode(err); status != 12 || !strings.Contains(stderr, "name not matched: nope") || !strings.Contains(fmt.Sprint(err), "Nothing to do!") {
		t.Errorf("zip none.zip nope = %v, %q", err, stderr)
	}
}
