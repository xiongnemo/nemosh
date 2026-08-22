package applets_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The paths ar and tar had left: member selection, the two index members an ar
// archive carries, tar's creation side, and the errors each reports.

// The symbol table `/` and the long-name table `//` are index, not content, so
// neither is listed or extracted. Listing them would invent two files that are not
// in the archive.
func TestAr_skipsTheIndexMembers(t *testing.T) {
	root := t.TempDir()
	longName := "a-name-far-too-long-for-sixteen-columns.txt"
	table := longName + "/\n"
	var archive bytes.Buffer
	archive.WriteString("!<arch>\n")
	// The symbol table comes first in a real library, holding an index nothing here
	// reads. Its body must still be skipped exactly or every offset after it moves.
	archive.WriteString(arTestHeader("/", 0, 0, 8))
	archive.WriteString("SYMINDEX")
	archive.WriteString(arTestHeader("//", 0, 0, len(table)))
	archive.WriteString(table)
	if len(table)%2 == 1 {
		archive.WriteString("\n")
	}
	archive.WriteString(arTestHeader("/0", 1700000000, 0o644, 6))
	archive.WriteString("member")
	if err := os.WriteFile(filepath.Join(root, "lib.a"), archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, root, "", "ar", "t", "lib.a")
	if err != nil {
		t.Fatalf("ar t: %v (%s)", err, stderr)
	}
	if stdout != longName+"\n" {
		t.Fatalf("ar t = %q, want only the real member", stdout)
	}
	// Extraction skips them too, so no file called `/` or `//` appears -- and the
	// real member's content proves both index bodies were consumed exactly.
	if _, stderr, err = runSmall(t, root, "", "ar", "x", "lib.a"); err != nil {
		t.Fatalf("ar x: %v (%s)", err, stderr)
	}
	got, err := os.ReadFile(filepath.Join(root, longName))
	if err != nil {
		t.Fatalf("the real member was not extracted: %v", err)
	}
	if string(got) != "member" {
		t.Fatalf("the member holds %q: an index body was mis-skipped", got)
	}
}

// A member operand selects, by exact name or by base name -- the second because
// `ar x lib.a src/a.txt` is a natural thing to type for an archive that stores
// `a.txt`.
func TestAr_selectsNamedMembers(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"a.txt": "alpha", "b.txt": "beta", "c.txt": "gamma"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, stderr, err := runSmall(t, root, "", "ar", "r", "out.a", "a.txt", "b.txt", "c.txt"); err != nil {
		t.Fatalf("ar r: %v (%s)", err, stderr)
	}

	stdout, _, err := runSmall(t, root, "", "ar", "p", "out.a", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "beta" {
		t.Fatalf("ar p out.a b.txt = %q, want just beta", stdout)
	}
	// A path whose base name matches selects too.
	if stdout, _, err = runSmall(t, root, "", "ar", "p", "out.a", "some/dir/c.txt"); err != nil {
		t.Fatal(err)
	}
	if stdout != "gamma" {
		t.Fatalf("selection by base name = %q, want gamma", stdout)
	}
	// Two operands select two, in archive order rather than operand order.
	if stdout, _, err = runSmall(t, root, "", "ar", "t", "out.a", "c.txt", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if stdout != "a.txt\nc.txt\n" {
		t.Fatalf("ar t with two operands = %q, want archive order", stdout)
	}
	// A name that is not in the archive selects nothing and is not an error, which
	// is both references' behaviour.
	if stdout, _, err = runSmall(t, root, "", "ar", "t", "out.a", "nope.txt"); err != nil {
		t.Fatalf("ar t with an absent member: %v", err)
	}
	if stdout != "" {
		t.Fatalf("an absent member selected %q", stdout)
	}
}

// -o restores the stored mtime on extraction, and without it the file gets now.
func TestAr_restoresTheMtimeOnlyWithDashO(t *testing.T) {
	root := t.TempDir()
	var archive bytes.Buffer
	archive.WriteString("!<arch>\n")
	old := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	archive.WriteString(arTestHeader("aged.txt/", int(old.Unix()), 0o644, 5))
	archive.WriteString("aged")
	archive.WriteString("\n")
	if err := os.WriteFile(filepath.Join(root, "a.a"), archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name    string
		args    []string
		wantOld bool
	}{
		{name: "with -o", args: []string{"x", "-o", "a.a"}, wantOld: true},
		{name: "without -o", args: []string{"x", "a.a"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := t.TempDir()
			source, err := os.ReadFile(filepath.Join(root, "a.a"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(target, "a.a"), source, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, stderr, err := runSmall(t, target, "", "ar", test.args...); err != nil {
				t.Fatalf("ar %v: %v (%s)", test.args, err, stderr)
			}
			info, err := os.Stat(filepath.Join(target, "aged.txt"))
			if err != nil {
				t.Fatal(err)
			}
			carried := info.ModTime().Before(time.Now().Add(-24 * time.Hour))
			if carried != test.wantOld {
				t.Fatalf("mtime %v: carried = %v, want %v", info.ModTime(), carried, test.wantOld)
			}
		})
	}
}

// -v names each member on stderr while `p` writes content to stdout, so
// `ar pv lib.a > out` does not put progress lines into the file.
func TestAr_verboseStaysOffStdout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runSmall(t, root, "", "ar", "rv", "out.a", "a.txt")
	if err != nil {
		t.Fatalf("ar rv: %v (%s)", err, stderr)
	}
	if !strings.Contains(stderr, "a.txt") {
		t.Fatalf("-v named nothing: %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("ar rv wrote %q to stdout", stdout)
	}

	stdout, stderr, err = runSmall(t, root, "", "ar", "pv", "out.a")
	if err != nil {
		t.Fatalf("ar pv: %v (%s)", err, stderr)
	}
	if stdout != "content" {
		t.Fatalf("ar pv stdout = %q, want only the content", stdout)
	}
}

// The errors ar reports rather than the ones it hides.
func TestAr_reportsWhatItCannotDo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name, because string
		args          []string
	}{
		{name: "an archive that is not there", args: []string{"t", "nope.a"}, because: "nope.a"},
		{name: "a file that is not an ar archive", args: []string{"t", "plain.txt"}, because: "ar archive"},
		{name: "no archive operand", args: []string{"t"}, because: "archive"},
		{name: "creating with nothing to archive", args: []string{"r", "out.a"}, because: "file"},
		{name: "archiving a directory", args: []string{"r", "out.a", "adir"}, because: "directory"},
		{name: "archiving a file that is not there", args: []string{"r", "out.a", "nope.txt"}, because: "nope.txt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := runSmall(t, root, "", "ar", test.args...)
			if err == nil {
				t.Fatalf("ar %v was accepted", test.args)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("ar %v said %q, which does not mention %q", test.args, err, test.because)
			}
		})
	}
	// A truncated archive: the magic is right and the header is cut short.
	if err := os.WriteFile(filepath.Join(root, "cut.a"), []byte("!<arch>\nname/     0"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := runSmall(t, root, "", "ar", "t", "cut.a")
	if err == nil || !strings.Contains(err.Error(), "part-way") {
		t.Fatalf("a truncated ar header said %v", err)
	}
	// And a header whose end marker is wrong, which is the only structural check
	// the format offers.
	bad := "!<arch>\n" + "name/           0     0     0     644     1         XX" + "y"
	if err := os.WriteFile(filepath.Join(root, "marker.a"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = runSmall(t, root, "", "ar", "t", "marker.a"); err == nil {
		t.Fatal("a header with the wrong end marker was accepted")
	}
}

// A member that claims more bytes than the archive holds is a truncation, and the
// partial file must not be left behind looking complete.
func TestAr_refusesATruncatedMemberAndLeavesNoPartial(t *testing.T) {
	root := t.TempDir()
	var archive bytes.Buffer
	archive.WriteString("!<arch>\n")
	// The header promises 100 bytes; four follow.
	archive.WriteString(arTestHeader("short.txt/", 0, 0o644, 100))
	archive.WriteString("only")
	if err := os.WriteFile(filepath.Join(root, "a.a"), archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := runSmall(t, root, "", "ar", "x", "a.a")
	if err == nil {
		t.Fatal("a truncated member was accepted")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("the error does not say truncated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "short.txt")); err == nil {
		t.Fatal("the truncated member left a partial file behind")
	}
}

// tar's creation side: -c with -C, a directory walked recursively, and -z.
func TestTar_createsFromADirectoryWalk(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		filepath.Join("src", "a.txt"):         "alpha\n",
		filepath.Join("src", "deep", "b.txt"): "beta\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, stderr, err := runSmall(t, root, "", "tar", "-cf", "out.tar", "src"); err != nil {
		t.Fatalf("tar -cf: %v (%s)", err, stderr)
	}
	stdout, stderr, err := runSmall(t, root, "", "tar", "-tf", "out.tar")
	if err != nil {
		t.Fatalf("tar -tf: %v (%s)", err, stderr)
	}
	// Both files are there, which is what says the walk descended. The directory
	// entries themselves are listed too; their exact spelling is not asserted
	// because a trailing slash is a writer's choice.
	for _, want := range []string{"src/a.txt", "src/deep/b.txt"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("tar -tf does not list %s: %q", want, stdout)
		}
	}
	// Extracted into a fresh directory, the tree comes back.
	target := t.TempDir()
	source, err := os.ReadFile(filepath.Join(root, "out.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "out.tar"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, target, "", "tar", "-xf", "out.tar"); err != nil {
		t.Fatalf("tar -xf: %v (%s)", err, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(target, "src", "deep", "b.txt")); err != nil || string(got) != "beta\n" {
		t.Fatalf("the deep file came back as %q (%v)", got, err)
	}
}

// -z compresses on the way out and decompresses on the way in, using this build's
// own gzip -- which is why `tar -czf` needs no second program.
func TestTar_roundTripsThroughItsOwnGzip(t *testing.T) {
	root := t.TempDir()
	// Repetitive, so the compressed form is unmistakably smaller than the plain one.
	content := strings.Repeat("compressible content\n", 200)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runSmall(t, root, "", "tar", "-czf", "out.tgz", "big.txt"); err != nil {
		t.Fatalf("tar -czf: %v (%s)", err, stderr)
	}
	if _, stderr, err := runSmall(t, root, "", "tar", "-cf", "out.tar", "big.txt"); err != nil {
		t.Fatalf("tar -cf: %v (%s)", err, stderr)
	}
	compressed, err := os.Stat(filepath.Join(root, "out.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := os.Stat(filepath.Join(root, "out.tar"))
	if err != nil {
		t.Fatal(err)
	}
	if compressed.Size() >= plain.Size() {
		t.Fatalf("-z produced %d bytes against %d plain, so nothing was compressed",
			compressed.Size(), plain.Size())
	}

	// -a chooses the compression from the archive name, so this reads the same file
	// without being told it is gzip.
	target := t.TempDir()
	source, err := os.ReadFile(filepath.Join(root, "out.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "out.tgz"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, target, "", "tar", "-xaf", "out.tgz"); err != nil {
		t.Fatalf("tar -xaf: %v (%s)", err, stderr)
	}
	got, err := os.ReadFile(filepath.Join(target, "big.txt"))
	if err != nil || string(got) != content {
		t.Fatalf("the round trip lost content: %d bytes back, %d out (%v)", len(got), len(content), err)
	}
}

// -O writes to stdout instead of to disk, and -C changes directory first.
func TestTar_extractToStdoutAndIntoADirectory(t *testing.T) {
	root := t.TempDir()
	archive := buildTar(t, map[string]string{"a.txt": "to stdout\n"})
	if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "into"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, root, "", "tar", "-xOf", "a.tar")
	if err != nil {
		t.Fatalf("tar -xOf: %v (%s)", err, stderr)
	}
	if stdout != "to stdout\n" {
		t.Fatalf("tar -xOf = %q", stdout)
	}
	// -O means nothing was written, which is the half worth asserting.
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("-O wrote a file as well as stdout")
	}

	if _, stderr, err = runSmall(t, root, "", "tar", "-xf", "a.tar", "-C", "into"); err != nil {
		t.Fatalf("tar -xf -C: %v (%s)", err, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(root, "into", "a.txt")); err != nil || string(got) != "to stdout\n" {
		t.Fatalf("-C did not extract into the directory: %q (%v)", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("-C extracted into the working directory as well")
	}
}

// The errors tar reports.
func TestTar_reportsWhatItCannotDo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("not a tar"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A real archive, so the bad -C case fails for the directory rather than for a
	// missing archive -- which is what the first draft of this test measured.
	if err := os.WriteFile(filepath.Join(root, "real.tar"), buildTar(t, map[string]string{"a.txt": "x"}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, because string
		args          []string
	}{
		{name: "no operation", args: []string{"-f", "a.tar"}, because: "exactly one"},
		// Two operations used to *choose* one: a switch on the three letters in
		// order meant `tar -c -x` created the archive and ignored the -x. busybox
		// refuses the same invocation, and so does this applet's sibling cpio.
		{name: "two operations", args: []string{"-c", "-x", "-f", "a.tar", "plain.txt"}, because: "exactly one"},
		{name: "all three operations", args: []string{"-ctx", "-f", "a.tar", "plain.txt"}, because: "exactly one"},
		{name: "an archive that is not there", args: []string{"-tf", "nope.tar"}, because: "nope.tar"},
		{name: "a file that is not a tar archive", args: []string{"-tf", "plain.txt"}, because: "tar"},
		{name: "creating with nothing to archive", args: []string{"-cf", "out.tar"}, because: "no files"},
		{name: "a -C directory that is not there", args: []string{"-xf", "real.tar", "-C", "nope"}, because: "nope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := runSmall(t, root, "", "tar", test.args...)
			if err == nil {
				t.Fatalf("tar %v was accepted", test.args)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("tar %v said %q, which does not mention %q", test.args, err, test.because)
			}
		})
	}
}

// A tar entry whose size disagrees with its body is a truncation, and archive/tar
// reports it -- what matters is that nothing partial is left looking whole.
func TestTar_leavesNoPartialFileOnATruncatedArchive(t *testing.T) {
	root := t.TempDir()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	header := &tar.Header{Name: "a.txt", Mode: 0o644, Size: 100, Typeflag: tar.TypeReg}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("only four")); err != nil {
		t.Fatal(err)
	}
	// Closed without the promised bytes, then cut short so the archive is genuinely
	// truncated rather than merely padded.
	writer.Flush()
	cut := buffer.Bytes()
	if err := os.WriteFile(filepath.Join(root, "cut.tar"), cut[:len(cut)-10], 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := runSmall(t, root, "", "tar", "-xf", "cut.tar")
	if err == nil {
		t.Fatal("a truncated tar archive was accepted")
	}
	info, statErr := os.Stat(filepath.Join(root, "a.txt"))
	if statErr == nil && info.Size() == 100 {
		t.Fatal("a partial file was left at its promised size, so it looks complete")
	}
}

// Listing shows hostile names unchecked, because inspecting an archive one does
// not trust is exactly when the hostile name must be visible -- and extraction of
// the same archive refuses them. The two halves are one decision.
func TestTar_listingAndExtractionDisagreeOnPurpose(t *testing.T) {
	root := t.TempDir()
	archive := buildTar(t, map[string]string{"../escape.txt": "x", "NUL": "y", "honest.txt": "z"})
	if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runSmall(t, root, "", "tar", "-tf", "a.tar")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape.txt", "NUL", "honest.txt"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("tar -tf hid %q, which is what a listing is for: %q", name, stdout)
		}
	}
	_, stderr, err := runSmall(t, root, "", "tar", "-xf", "a.tar")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stderr, "skipping") != 2 {
		t.Fatalf("extraction skipped %d entries, want 2: %q", strings.Count(stderr, "skipping"), stderr)
	}
}
