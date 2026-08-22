package applets_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What cpio and ar do when the archive is honest, and what they say when it is
// not. The containment cases live in cpio_ar_test.go beside the shared hostile
// name table; these are the format's own edges -- padding, a missing trailer, a
// name that does not fit its column.

// A cpio archive made here is read back here, and the listing matches what
// busybox prints for the same archive -- measured, not guessed: mode, uid/gid, a
// nine-wide size, a full timestamp.
func TestCpio_roundTripsAndListsLikeBusybox(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"a.txt": "alpha\n", "b.txt": "beta beta\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// When: created from a name list on stdin, which is what makes cpio cpio
	stdout, stderr, err := runSmall(t, root, "a.txt\nb.txt\n", "cpio", "-o", "-H", "newc", "-F", "out.cpio")
	if err != nil {
		t.Fatalf("cpio -o: %v (%s)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("cpio -o -F wrote %q to stdout, which should have gone to the file", stdout)
	}
	// Both references end with this on stderr. It is how a caller knows the whole
	// archive was written when every member was silent.
	if !strings.Contains(stderr, "blocks") {
		t.Fatalf("cpio -o did not report a block count: %q", stderr)
	}

	// Then: it reads back, in order, with the sizes it stored
	stdout, stderr, err = runSmall(t, root, "", "cpio", "-tv", "-F", "out.cpio")
	if err != nil {
		t.Fatalf("cpio -tv: %v (%s)", err, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("cpio -tv listed %d entries: %q", len(lines), stdout)
	}
	for index, want := range []struct{ size, name string }{{"6", "a.txt"}, {"10", "b.txt"}} {
		// mode, uid/gid, size, date, time, name
		fields := strings.Fields(lines[index])
		if len(fields) != 6 {
			t.Fatalf("line %q has %d columns, want 6", lines[index], len(fields))
		}
		if fields[0] != "-rw-r--r--" {
			t.Errorf("mode column = %q, want -rw-r--r--", fields[0])
		}
		if fields[2] != want.size || fields[5] != want.name {
			t.Errorf("line %q: size %q name %q, want %q and %q",
				lines[index], fields[2], fields[5], want.size, want.name)
		}
	}

	// And the contents survive a round trip into a fresh directory.
	nested := filepath.Join(root, "back")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "out.cpio"), filepath.Join(nested, "out.cpio")); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err = runSmall(t, nested, "", "cpio", "-i", "-d", "-F", "out.cpio"); err != nil {
		t.Fatalf("cpio -i: %v (%s)", err, stderr)
	}
	for name, want := range map[string]string{"a.txt": "alpha\n", "b.txt": "beta beta\n"} {
		got, err := os.ReadFile(filepath.Join(nested, name))
		if err != nil {
			t.Fatalf("%s was not extracted: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// The three modes are exclusive and one is required. Defaulting to a list would
// make cpio with a typo look like it did nothing rather than like it was asked
// nothing.
func TestCpio_requiresExactlyOneMode(t *testing.T) {
	for _, args := range [][]string{{"cpio"}, {"cpio", "-v"}, {"cpio", "-t", "-i"}, {"cpio", "-tio"}} {
		if _, _, err := runSmall(t, t.TempDir(), "", args[0], args[1:]...); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

// Only newc is read. An older header is refused by its magic rather than misread,
// because the octal and raw-binary layouts put different things in the same bytes.
func TestCpio_refusesAFormatItCannotRead(t *testing.T) {
	root := t.TempDir()
	// The POSIX portable format: a 76-byte octal ASCII header beginning 070707.
	octal := "070707" + strings.Repeat("0", 70) + "a.txt\x00"
	if err := os.WriteFile(filepath.Join(root, "old.cpio"), []byte(octal), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "old.cpio")
	if err == nil {
		t.Fatal("an odc archive was accepted")
	}
	if !strings.Contains(err.Error(), "070707") {
		t.Fatalf("the refusal does not quote the magic it found: %v", err)
	}
}

// A truncated archive is a truncated archive. cpio has an explicit trailer, so its
// absence is knowable rather than a guess -- and saying so is the difference
// between "the archive ended" and "the archive was cut off".
func TestCpio_reportsAMissingTrailer(t *testing.T) {
	root := t.TempDir()
	full := buildCpio(t, []cpioTestEntry{{name: "a.txt", content: "alpha\n"}})
	// Everything but the trailer entry, whose header and padded name are 124 bytes.
	if err := os.WriteFile(filepath.Join(root, "cut.cpio"), full[:len(full)-124], 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runSmall(t, root, "", "cpio", "-t", "-F", "cut.cpio")
	if err == nil {
		t.Fatal("a truncated archive was accepted")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("the refusal does not say the archive is truncated: %v", err)
	}
}

// ar round-trips, and stores the base name: the format has no directories, so
// `ar r out.a src/a.txt` holds a.txt and both references agree.
func TestAr_roundTripsAndFlattensNames(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	// An odd length, so the padding byte is exercised: a member that does not pad
	// shifts every header after it.
	if err := os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte("odd"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("even"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runSmall(t, root, "", "ar", "r", "out.a", "src/a.txt", "b.txt"); err != nil {
		t.Fatalf("ar r: %v (%s)", err, stderr)
	}
	stdout, stderr, err := runSmall(t, root, "", "ar", "t", "out.a")
	if err != nil {
		t.Fatalf("ar t: %v (%s)", err, stderr)
	}
	if stdout != "a.txt\nb.txt\n" {
		t.Fatalf("ar t = %q, want the flattened names in order", stdout)
	}
	// p concatenates, which is how a missing padding byte shows up: an unpadded
	// odd member makes the next header start one byte early.
	stdout, stderr, err = runSmall(t, root, "", "ar", "p", "out.a")
	if err != nil {
		t.Fatalf("ar p: %v (%s)", err, stderr)
	}
	if stdout != "oddeven" {
		t.Fatalf("ar p = %q, want oddeven", stdout)
	}
}

// The verb is the first word and only the first word. This is the regression test
// for reading a p out of the middle of a Windows temporary path and calling it the
// operation.
func TestAr_takesItsVerbFromTheFirstWordOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "path.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// path.txt holds a p, a t and an x. As an operand it must stay an operand.
	if _, stderr, err := runSmall(t, root, "", "ar", "r", "out.a", "path.txt"); err != nil {
		t.Fatalf("ar r with a verb-shaped operand: %v (%s)", err, stderr)
	}
	stdout, _, err := runSmall(t, root, "", "ar", "t", "out.a")
	if err != nil || stdout != "path.txt\n" {
		t.Fatalf("ar t = %q (%v)", stdout, err)
	}
	// A first word with no verb in it is refused rather than falling through to
	// some default operation.
	for _, args := range [][]string{{"ar", "out.a"}, {"ar", "-v", "out.a"}, {"ar"}} {
		if _, _, err := runSmall(t, root, "", args[0], args[1:]...); err == nil {
			t.Errorf("%v was accepted with no verb", args)
		}
	}
	// And an unknown letter in the first word names itself.
	_, _, err = runSmall(t, root, "", "ar", "-Z", "out.a")
	if err == nil || !strings.Contains(err.Error(), "Z") {
		t.Fatalf("ar -Z said %v, want the letter named", err)
	}
}

// -r creates and refuses to touch an existing archive. Appending correctly means
// rewriting the long-name table, and a half-done version corrupts the archive it
// was given -- so the limit is stated rather than approximated.
func TestAr_refusesToAddToAnExistingArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, root, "", "ar", "r", "out.a", "a.txt"); err != nil {
		t.Fatalf("ar r: %v (%s)", err, stderr)
	}
	before, err := os.ReadFile(filepath.Join(root, "out.a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, root, "", "ar", "r", "out.a", "a.txt"); err == nil {
		t.Fatal("ar r overwrote an existing archive")
	}
	after, err := os.ReadFile(filepath.Join(root, "out.a"))
	if err != nil {
		t.Fatal(err)
	}
	// The refusal has to leave the archive alone, which is the whole point of it.
	if !bytes.Equal(before, after) {
		t.Fatal("the refused ar r changed the archive anyway")
	}
}

// A name too long for the sixteen columns is refused, not truncated: a truncated
// name is a different file, written silently.
func TestAr_refusesANameTooLongForAHeader(t *testing.T) {
	root := t.TempDir()
	long := "a-name-far-too-long-for-sixteen-columns.txt"
	if err := os.WriteFile(filepath.Join(root, long), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, root, "", "ar", "r", "out.a", long); err == nil {
		t.Fatal("a name too long for an ar header was accepted")
	}
	// And the half-written archive is gone: one that looks real and is not is
	// worse than none at all.
	if _, err := os.Stat(filepath.Join(root, "out.a")); err == nil {
		t.Fatal("the failed ar r left an archive behind")
	}
}

// The long-name table is read even though it is never written, because a .deb or a
// library made by GNU ar will have one.
func TestAr_readsTheLongNameTable(t *testing.T) {
	root := t.TempDir()
	long := "a-name-far-too-long-for-sixteen-columns.txt"
	table := long + "/\n"
	var archive bytes.Buffer
	archive.WriteString("!<arch>\n")
	archive.WriteString(arTestHeader("//", 0, 0, len(table)))
	archive.WriteString(table)
	if len(table)%2 == 1 {
		archive.WriteString("\n")
	}
	// A member naming offset 0 in that table.
	archive.WriteString(arTestHeader("/0", 1700000000, 0o644, 5))
	archive.WriteString("hello")
	archive.WriteString("\n")
	if err := os.WriteFile(filepath.Join(root, "long.a"), archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, root, "", "ar", "t", "long.a")
	if err != nil {
		t.Fatalf("ar t: %v (%s)", err, stderr)
	}
	if stdout != long+"\n" {
		t.Fatalf("ar t = %q, want the long name resolved to %q", stdout, long)
	}
	// The table itself is not a member: listing it would invent a file called //.
	if strings.Contains(stdout, "//") {
		t.Fatal("the long-name table was listed as a member")
	}
}

func arTestHeader(stored string, mtime, mode, size int) string {
	return fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n", stored, mtime, 0, 0, mode, size)
}
