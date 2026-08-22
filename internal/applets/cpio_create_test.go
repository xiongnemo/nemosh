package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cpio's creation side and its per-entry options, which the round-trip test does
// not reach: it archives two plain writable files and reads them back, so the
// three other entry shapes and every option that changes what lands on disk were
// uncovered.

// A directory in the name list becomes a directory entry: a mode bit and no data.
// Getting the "no data" part wrong misaligns every entry after it, so this asserts
// the file that follows as well.
func TestCpio_archivesADirectoryEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("inside\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runSmall(t, root, "sub\nsub/a.txt\n", "cpio", "-o", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	stdout, stderr, err := runSmall(t, root, "", "cpio", "-tv", "-F", "out.cpio")
	if err != nil {
		t.Fatalf("cpio -tv: %v (%s)", err, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("cpio -tv listed %d entries: %q", len(lines), stdout)
	}
	// The mode column's first character is the entry type, and a directory reads
	// as `d`. That is the only place the type is visible in a listing.
	if !strings.HasPrefix(lines[0], "d") {
		t.Errorf("the directory listed as %q, want a leading d", lines[0])
	}
	if !strings.HasPrefix(lines[1], "-") {
		t.Errorf("the file listed as %q, want a leading dash", lines[1])
	}
	// Size zero for the directory, and the file that follows is intact -- which is
	// what proves the directory entry carried no body.
	if fields := strings.Fields(lines[0]); fields[2] != "0" {
		t.Errorf("the directory entry claims %s bytes, want 0", fields[2])
	}
	if fields := strings.Fields(lines[1]); fields[2] != "7" || fields[5] != "sub/a.txt" {
		t.Errorf("the entry after the directory read %q", lines[1])
	}
}

// A read-only file gets 0444 rather than Go's own mode bits. Windows has no
// execute bit and no group or other permissions, so the mode is synthesised, and
// writing os.FileMode into a Unix mode field would be a misreport.
func TestCpio_synthesisesTheModeItStores(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rw.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ro.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "ro.txt"), 0o444); err != nil {
		t.Fatal(err)
	}
	// Restored so the temporary directory can be removed on every platform.
	t.Cleanup(func() { os.Chmod(filepath.Join(root, "ro.txt"), 0o600) })

	if _, stderr, err := runSmall(t, root, "rw.txt\nro.txt\n", "cpio", "-o", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	stdout, _, err := runSmall(t, root, "", "cpio", "-tv", "-F", "out.cpio")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("listed %d entries: %q", len(lines), stdout)
	}
	if got := strings.Fields(lines[0])[0]; got != "-rw-r--r--" {
		t.Errorf("the writable file stored mode %q, want -rw-r--r--", got)
	}
	if got := strings.Fields(lines[1])[0]; got != "-r--r--r--" {
		t.Errorf("the read-only file stored mode %q, want -r--r--r--", got)
	}
}

// -0 reads NUL-separated names, which is how `find -print0` hands over a name
// with a newline in it. The newline is the whole reason the option exists, so the
// fixture has one.
func TestCpio_readsNulSeparatedNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Without -0 the same input would be four names, two of them nonexistent.
	if _, stderr, err := runSmall(t, root, "a.txt\x00b.txt\x00", "cpio", "-o", "-0", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o0: %v (%s)", err, stderr)
	}
	stdout, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "out.cpio")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "a.txt\nb.txt\n" {
		t.Fatalf("cpio -t = %q, want both names", stdout)
	}

	// And a final name with no separator after it is still a name, the way a last
	// line without a newline is.
	if _, stderr, err := runSmall(t, root, "a.txt\x00b.txt", "cpio", "-o", "-0", "-F", "two.cpio"); err != nil {
		t.Fatalf("cpio -o0 without a trailing NUL: %v (%s)", err, stderr)
	}
	if stdout, _, err = runSmall(t, root, "", "cpio", "-t", "-F", "two.cpio"); err != nil {
		t.Fatal(err)
	}
	if stdout != "a.txt\nb.txt\n" {
		t.Fatalf("a name without a trailing NUL was dropped: %q", stdout)
	}
}

// A blank line in the name list is skipped rather than becoming an error about a
// file called "". A list built by a shell pipeline routinely ends in one.
func TestCpio_skipsBlankNames(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Including a CRLF line, because a list built on this platform is very likely
	// to have them and "a.txt\r" is not a file that exists.
	if _, stderr, err := runSmall(t, root, "\na.txt\r\n\n", "cpio", "-o", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	stdout, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "out.cpio")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "a.txt\n" {
		t.Fatalf("cpio -t = %q, want just a.txt", stdout)
	}
}

// A name that does not exist is an error, not a silent omission: an archive
// missing a file the caller asked for should not look like a success.
func TestCpio_reportsANameItCannotRead(t *testing.T) {
	root := t.TempDir()
	if _, _, err := runSmall(t, root, "missing.txt\n", "cpio", "-o", "-F", "out.cpio"); err == nil {
		t.Fatal("cpio -o accepted a name that does not exist")
	}
}

// -m restores the stored mtime; without it the extracted file gets now, which is
// what both references do -- the mtime in the archive is information about the
// original and only -m says to carry it over.
func TestCpio_restoresTheMtimeOnlyWithDashM(t *testing.T) {
	source := t.TempDir()
	old := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("aged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(source, "a.txt"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, source, "a.txt\n", "cpio", "-o", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	archive, err := os.ReadFile(filepath.Join(source, "out.cpio"))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name    string
		args    []string
		wantOld bool
	}{
		{name: "with -m", args: []string{"cpio", "-i", "-m", "-F", "a.cpio"}, wantOld: true},
		{name: "without -m", args: []string{"cpio", "-i", "-F", "a.cpio"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			if err := os.WriteFile(filepath.Join(target, "a.cpio"), archive, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, stderr, err := runSmall(t, target, "", test.args[0], test.args[1:]...); err != nil {
				t.Fatalf("%v: %v (%s)", test.args, err, stderr)
			}
			info, err := os.Stat(filepath.Join(target, "a.txt"))
			if err != nil {
				t.Fatal(err)
			}
			// A whole day of slack: the question is which of two times it is, and
			// they are three days apart.
			carried := info.ModTime().Before(time.Now().Add(-24 * time.Hour))
			if carried != test.wantOld {
				t.Fatalf("mtime is %v; carried over = %v, want %v", info.ModTime(), carried, test.wantOld)
			}
		})
	}
}

// An existing file is left alone without -u, and said so. There is no prompt to
// ask at, so the safe half of busybox's interactive choice is taken and named.
func TestCpio_leavesAnExistingFileAloneWithoutDashU(t *testing.T) {
	root := t.TempDir()
	archive := buildCpio(t, []cpioTestEntry{{name: "a.txt", content: "from the archive\n"}})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("already here\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "cpio", "-i", "-F", "a.cpio")
	if err != nil {
		t.Fatalf("cpio -i: %v (%s)", err, stderr)
	}
	if !strings.Contains(stderr, "-u") {
		t.Fatalf("cpio did not say how to overwrite: %q", stderr)
	}
	got, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil || string(got) != "already here\n" {
		t.Fatalf("the existing file was replaced: %q", got)
	}

	// And -u replaces it.
	if _, stderr, err = runSmall(t, root, "", "cpio", "-i", "-u", "-F", "a.cpio"); err != nil {
		t.Fatalf("cpio -i -u: %v (%s)", err, stderr)
	}
	if got, err = os.ReadFile(filepath.Join(root, "a.txt")); err != nil || string(got) != "from the archive\n" {
		t.Fatalf("-u did not replace the file: %q", got)
	}
}

// A pattern selects a subset, and the entries it does not select still have their
// bodies consumed -- otherwise the stream loses alignment and the next selected
// entry reads the previous one's data.
func TestCpio_selectsBbyPatternAndStaysAligned(t *testing.T) {
	root := t.TempDir()
	archive := buildCpio(t, []cpioTestEntry{
		{name: "skip1.log", content: "aaaaaaaaaa"},
		{name: "want.txt", content: "the wanted bytes\n"},
		{name: "skip2.log", content: "bbb"},
		{name: "also.txt", content: "also wanted\n"},
	})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, root, "", "cpio", "-t", "-F", "a.cpio", "*.txt")
	if err != nil {
		t.Fatalf("cpio -t with a pattern: %v (%s)", err, stderr)
	}
	if stdout != "want.txt\nalso.txt\n" {
		t.Fatalf("cpio -t '*.txt' = %q", stdout)
	}

	if _, stderr, err = runSmall(t, root, "", "cpio", "-i", "-F", "a.cpio", "*.txt"); err != nil {
		t.Fatalf("cpio -i with a pattern: %v (%s)", err, stderr)
	}
	// Both wanted files are correct, which is what proves the skipped bodies were
	// consumed exactly.
	for name, want := range map[string]string{"want.txt": "the wanted bytes\n", "also.txt": "also wanted\n"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("%s was not extracted: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q: the stream lost alignment", name, got, want)
		}
	}
	for _, name := range []string{"skip1.log", "skip2.log"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("%s was extracted despite not matching", name)
		}
	}
	// An exact name is also a selector, not only a glob.
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, target, "", "cpio", "-i", "-F", "a.cpio", "skip2.log"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "skip2.log")); err != nil || string(got) != "bbb" {
		t.Fatalf("an exact name did not select: %q (%v)", got, err)
	}
}

// Without -d a missing parent directory is an error rather than being invented.
// An archive whose entries arrive before their directory is unusual enough that
// silently building the tree would hide a real problem.
func TestCpio_needsDashDForAMissingParent(t *testing.T) {
	root := t.TempDir()
	archive := buildCpio(t, []cpioTestEntry{{name: "deep/sub/a.txt", content: "x"}})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runSmall(t, root, "", "cpio", "-i", "-F", "a.cpio"); err == nil {
		t.Fatal("cpio -i invented a missing parent without -d")
	}
	if _, stderr, err := runSmall(t, root, "", "cpio", "-i", "-d", "-F", "a.cpio"); err != nil {
		t.Fatalf("cpio -i -d: %v (%s)", err, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "deep", "sub", "a.txt")); err != nil || string(got) != "x" {
		t.Fatalf("-d did not build the tree: %q (%v)", got, err)
	}
}

// -F on an archive that is not there is an error naming the operand, not a silent
// empty listing.
func TestCpio_reportsAnArchiveItCannotOpen(t *testing.T) {
	root := t.TempDir()
	_, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "nope.cpio")
	if err == nil {
		t.Fatal("cpio -t accepted an archive that is not there")
	}
	if !strings.Contains(err.Error(), "nope.cpio") {
		t.Fatalf("the error does not name the archive: %v", err)
	}
}

// -v names each entry on stderr while the listing goes to stdout, so `cpio -o -v`
// in a pipeline does not corrupt the archive with its own progress output.
func TestCpio_verboseWritesToStderrNotTheArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// No -F, so the archive goes to stdout and -v must not join it there.
	stdout, stderr, err := runSmall(t, root, "a.txt\n", "cpio", "-o", "-v")
	if err != nil {
		t.Fatalf("cpio -o -v: %v (%s)", err, stderr)
	}
	if !strings.Contains(stderr, "a.txt") {
		t.Fatalf("-v named nothing on stderr: %q", stderr)
	}
	if !strings.HasPrefix(stdout, "070701") {
		t.Fatalf("stdout does not begin with a cpio header: %q", stdout[:min(len(stdout), 32)])
	}
	// The archive stdout carries is readable, which is what proves -v stayed out
	// of it: one stray line would misalign the first header.
	if err := os.WriteFile(filepath.Join(root, "piped.cpio"), []byte(stdout), 0o600); err != nil {
		t.Fatal(err)
	}
	listing, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "piped.cpio")
	if err != nil {
		t.Fatalf("the piped archive is unreadable: %v", err)
	}
	if listing != "a.txt\n" {
		t.Fatalf("the piped archive listed %q", listing)
	}
}

// -H names the format, and anything but newc is refused rather than being written
// as newc under another name.
func TestCpio_refusesAFormatItCannotWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"odc", "crc", "ustar", "bin", ""} {
		_, _, err := runSmall(t, root, "a.txt\n", "cpio", "-o", "-H", format, "-F", "out.cpio")
		if err == nil {
			t.Errorf("cpio -o -H %q was accepted", format)
		}
	}
	// And newc is accepted, so the check is not simply refusing every -H.
	if _, stderr, err := runSmall(t, root, "a.txt\n", "cpio", "-o", "-H", "newc", "-F", "ok.cpio"); err != nil {
		t.Fatalf("cpio -o -H newc: %v (%s)", err, stderr)
	}
}

// A symlink in the name list is stored as a symlink entry with its target as the
// data. Creating one needs a privilege on Windows, so the test says why it
// skipped rather than passing vacuously.
func TestCpio_archivesASymlinkEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("pointed at\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "link")); err != nil {
		t.Skipf("this machine cannot create a symlink, which Windows needs a privilege for: %v", err)
	}

	if _, stderr, err := runSmall(t, root, "link\ntarget.txt\n", "cpio", "-o", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	stdout, _, err := runSmall(t, root, "", "cpio", "-tv", "-F", "out.cpio")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("listed %d entries: %q", len(lines), stdout)
	}
	// `l` is the type character for a symlink, and the size is the target's length
	// -- which is what says the target was stored as the entry's data.
	if !strings.HasPrefix(lines[0], "l") {
		t.Errorf("the symlink listed as %q, want a leading l", lines[0])
	}
	if fields := strings.Fields(lines[0]); fields[2] != "10" {
		t.Errorf("the symlink entry claims %s bytes, want 10 for %q", fields[2], "target.txt")
	}
}
