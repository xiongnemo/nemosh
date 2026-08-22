package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bunzip2 and bzcat, which had none of this.
//
// They were registered, documented in the support matrix, and **run by nothing**:
// compress_test.go names them in its header comment and then tests only the gzip
// family, and `tar -j` had no test either, so the whole bzip2 decompression path
// was unexercised. It worked when finally tried by hand, which is the bad kind of
// luck -- a silent regression had nowhere to be caught.
//
// The fixture is a literal because **Go cannot compress bzip2**. The standard
// library decompresses it and has no writer, which is also why this build has no
// `bzip2` applet and leaves that name unregistered for a real one on PATH. So the
// input cannot be produced by the code under test, and a hand-made archive from
// the reference is the only honest way to test the reader: this one came from
// busybox-w32 v1.38.0 `bzip2 -c`.
var bzip2Fixture = []byte{
	// 62 bytes holding exactly "bzip2 through nemosh\n"
	0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x97, 0x2b,
	0x38, 0xdb, 0x00, 0x00, 0x02, 0x59, 0x80, 0x00, 0x10, 0x40, 0x00, 0x10,
	0x00, 0x12, 0xe3, 0xde, 0x10, 0x20, 0x00, 0x22, 0x86, 0x80, 0xc4, 0xf1,
	0x42, 0x86, 0x9a, 0x60, 0x03, 0x54, 0x68, 0x67, 0x21, 0xca, 0x98, 0x08,
	0x85, 0x9b, 0x2f, 0x8b, 0xb9, 0x22, 0x9c, 0x28, 0x48, 0x4b, 0x95, 0x9c,
	0x6d, 0x80,
}

const bzip2Plain = "bzip2 through nemosh\n"

// bzcat writes to stdout and leaves the archive alone, which is the whole point of
// the name: `bzcat log.bz2 | grep` is why a decompressor that only works on files
// is not enough.
func TestBzcat_readsAFileAndAPipe(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.txt.bz2"), bzip2Fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, dir, "", "bzcat", "p.txt.bz2")
	if err != nil {
		t.Fatalf("bzcat: %v (%s)", err, stderr)
	}
	if stdout != bzip2Plain {
		t.Fatalf("bzcat = %q, want %q", stdout, bzip2Plain)
	}
	// The archive is still there: bzcat reads, it does not consume.
	if _, err := os.Stat(filepath.Join(dir, "p.txt.bz2")); err != nil {
		t.Fatalf("bzcat removed its input: %v", err)
	}

	// And from a pipe, which is the case busybox's own zcat gets wrong -- it
	// seeks on its input, so `cat x.gz | busybox zcat` answers "Invalid seek".
	// This reads sequentially, so both work.
	stdout, stderr, err = runSmall(t, dir, string(bzip2Fixture), "bzcat")
	if err != nil {
		t.Fatalf("bzcat from a pipe: %v (%s)", err, stderr)
	}
	if stdout != bzip2Plain {
		t.Fatalf("bzcat from a pipe = %q, want %q", stdout, bzip2Plain)
	}
}

// bunzip2 replaces the file and removes the archive, which is both references'
// default and the behaviour that surprises people.
func TestBunzip2_replacesTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.txt.bz2"), bzip2Fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runSmall(t, dir, "", "bunzip2", "p.txt.bz2"); err != nil {
		t.Fatalf("bunzip2: %v (%s)", err, stderr)
	}

	got, err := os.ReadFile(filepath.Join(dir, "p.txt"))
	if err != nil {
		t.Fatalf("bunzip2 did not write p.txt: %v", err)
	}
	if string(got) != bzip2Plain {
		t.Fatalf("p.txt = %q, want %q", got, bzip2Plain)
	}
	// Removing the original is the half that differs on Windows: a file cannot be
	// deleted while a handle to it is open, and the first draft of the gzip code
	// held the source open across the remove.
	if _, err := os.Stat(filepath.Join(dir, "p.txt.bz2")); err == nil {
		t.Fatal("bunzip2 left the archive behind")
	}
}

// -k keeps it, and -c is the same as bzcat.
func TestBunzip2_keepAndStdout(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"k.txt.bz2", "c.txt.bz2"} {
		if err := os.WriteFile(filepath.Join(dir, name), bzip2Fixture, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, stderr, err := runSmall(t, dir, "", "bunzip2", "-k", "k.txt.bz2"); err != nil {
		t.Fatalf("bunzip2 -k: %v (%s)", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "k.txt.bz2")); err != nil {
		t.Fatalf("-k removed the archive anyway: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "k.txt")); err != nil {
		t.Fatalf("-k did not write the plain file: %v", err)
	}

	stdout, stderr, err := runSmall(t, dir, "", "bunzip2", "-c", "c.txt.bz2")
	if err != nil {
		t.Fatalf("bunzip2 -c: %v (%s)", err, stderr)
	}
	if stdout != bzip2Plain {
		t.Fatalf("bunzip2 -c = %q, want %q", stdout, bzip2Plain)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.txt")); err == nil {
		t.Fatal("-c wrote a file as well as stdout")
	}
}

// A corrupt archive is an error, not a short read. -t exists to answer exactly
// this, and it has to read the whole stream to do it.
func TestBunzip2_detectsCorruption(t *testing.T) {
	dir := t.TempDir()
	bad := append([]byte{}, bzip2Fixture...)
	// A byte in the middle of the compressed block, past the header, so the
	// failure is in the decoder rather than in the magic.
	bad[30] ^= 0xff
	if err := os.WriteFile(filepath.Join(dir, "bad.bz2"), bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.bz2"), bzip2Fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, stderr, err := runSmall(t, dir, "", "bunzip2", "-t", "good.bz2"); err != nil {
		t.Fatalf("bunzip2 -t on a good archive: %v (%s)", err, stderr)
	}
	if _, _, err := runSmall(t, dir, "", "bunzip2", "-t", "bad.bz2"); err == nil {
		t.Fatal("bunzip2 -t accepted a corrupt archive")
	}
	// And the corrupt archive must not leave a half-written plain file that looks
	// like a successful decompression.
	if _, _, err := runSmall(t, dir, "", "bunzip2", "bad.bz2"); err == nil {
		t.Fatal("bunzip2 accepted a corrupt archive")
	}
	if _, err := os.Stat(filepath.Join(dir, "bad")); err == nil {
		t.Fatal("the failed bunzip2 left a partial file behind")
	}
}

// Something that is not bzip2 at all is refused by name rather than producing
// nothing and exiting 0.
func TestBunzip2_refusesAFileThatIsNotBzip2(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.bz2"), []byte("not compressed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, dir, "", "bzcat", "plain.bz2"); err == nil {
		t.Fatal("bzcat accepted a file that is not bzip2")
	}
}

// tar -j, which had no test either. This is the reason bzip2 decompression is
// worth having at all on a machine whose own tar.exe cannot do it.
func TestTar_readsABzip2Archive(t *testing.T) {
	dir := t.TempDir()
	// A .tar holding one file, then bzip2 of that -- which cannot be built here,
	// so it is another literal from busybox: `tar -cjf` of a directory `src`
	// holding `a.txt` whose contents are "in tar\n".
	if err := os.WriteFile(filepath.Join(dir, "ref.tbz"), tarBzip2Fixture, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runSmall(t, dir, "", "tar", "-tjf", "ref.tbz")
	if err != nil {
		t.Fatalf("tar -tjf: %v (%s)", err, stderr)
	}
	if !strings.Contains(stdout, "src/a.txt") {
		t.Fatalf("tar -tjf listed %q, want src/a.txt", stdout)
	}

	if _, stderr, err := runSmall(t, dir, "", "tar", "-xjf", "ref.tbz"); err != nil {
		t.Fatalf("tar -xjf: %v (%s)", err, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "src", "a.txt"))
	if err != nil {
		t.Fatalf("tar -xjf did not extract: %v", err)
	}
	if string(got) != "in tar\n" {
		t.Fatalf("extracted %q, want %q", got, "in tar\n")
	}
}

// tarBzip2Fixture is 153 bytes: busybox-w32 `tar -cjf` of a directory `src`
// holding `a.txt` with one line of text in it. A literal for the same reason
// as bzip2Fixture -- Go has no bzip2 writer, so the input cannot come from the
// code under test.
var tarBzip2Fixture = []byte{
	0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x34, 0x43,
	0x0a, 0xad, 0x00, 0x00, 0x69, 0xfb, 0x84, 0xc1, 0x90, 0x00, 0x40, 0x40,
	0x01, 0xff, 0x80, 0x00, 0x84, 0x6a, 0x23, 0x9e, 0x40, 0x00, 0x00, 0x80,
	0x18, 0x20, 0x00, 0x94, 0x05, 0x54, 0x14, 0xf4, 0x11, 0xa3, 0x26, 0x86,
	0x21, 0xa1, 0x90, 0x4a, 0x4d, 0x41, 0xa6, 0x83, 0x4d, 0x34, 0x00, 0xd0,
	0x1b, 0x92, 0x75, 0x77, 0xce, 0x7c, 0x83, 0x9a, 0x01, 0x10, 0x60, 0x88,
	0xf8, 0x28, 0x61, 0x54, 0x47, 0xb6, 0xc2, 0x90, 0x80, 0xc0, 0x14, 0x92,
	0xa6, 0x92, 0x4d, 0x74, 0xf2, 0xa5, 0x44, 0x33, 0xd2, 0xb4, 0x26, 0x45,
	0x7c, 0x8e, 0x71, 0x4a, 0x69, 0xba, 0x22, 0xbb, 0x86, 0x8a, 0xaf, 0x92,
	0x2e, 0x34, 0xc2, 0xc5, 0x65, 0x26, 0xb6, 0x86, 0x53, 0x94, 0x13, 0x89,
	0x69, 0x23, 0xc6, 0x50, 0xc5, 0x89, 0xb7, 0xb3, 0x5a, 0xde, 0x9f, 0x0c,
	0xe1, 0xf5, 0xbd, 0x9b, 0x37, 0xca, 0x8d, 0xb7, 0x61, 0x70, 0xfe, 0x2e,
	0xe4, 0x8a, 0x70, 0xa1, 0x20, 0x68, 0x86, 0x15, 0x5a,
}
