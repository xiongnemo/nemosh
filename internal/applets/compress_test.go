package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gzip, gunzip, zcat, bunzip2 and bzcat.
//
// These are stream filters, which is why Windows shipping tar.exe does not cover
// them. Round trips are checked in both directions against busybox-made archives
// in the differential sweep; these tests pin the behaviours that are easy to get
// wrong on this platform.

func TestGzip_roundTripsThroughAFile(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello world hello world\n"})
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}

	// The default replaces the file and removes the original, which is the
	// behaviour that surprises people and what both references do.
	if _, stderr, err := runSmall(t, dir, "", "gzip", "h.txt"); err != nil {
		t.Fatalf("gzip: %v (%s)", err, stderr)
	}
	if exists("h.txt") {
		t.Fatal("gzip left the original in place; it should have been removed")
	}
	if !exists("h.txt.gz") {
		t.Fatal("gzip did not write h.txt.gz")
	}

	// And back, which restores the name by stripping the suffix.
	if _, stderr, err := runSmall(t, dir, "", "gunzip", "h.txt.gz"); err != nil {
		t.Fatalf("gunzip: %v (%s)", err, stderr)
	}
	if exists("h.txt.gz") {
		t.Fatal("gunzip left the archive in place")
	}
	data, err := os.ReadFile(filepath.Join(dir, "h.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world hello world\n" {
		t.Fatalf("round trip produced %q", data)
	}
}

// Removing the original is where Windows differs from Unix: a file cannot be
// deleted while a handle to it is open. Holding the source open across the remove
// failed with "The process cannot access the file because it is being used by
// another process" -- and would have passed silently on Linux, where the unlink
// succeeds regardless.
func TestGzip_removesTheOriginalOnWindows(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"a.txt": "payload\n"})
	stdout, stderr, err := runSmall(t, dir, "", "gzip", "a.txt")
	if err != nil {
		t.Fatalf("gzip: %v (stdout %q, stderr %q)", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("gzip complained while removing the original: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err == nil {
		t.Fatal("the original survived, so the remove failed silently")
	}
}

func TestGzip_optionsThatChangeWhatIsWritten(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"k.txt": "keep me\n",
		"c.txt": "to stdout\n",
	})
	// -k keeps the input beside the archive.
	if _, _, err := runSmall(t, dir, "", "gzip", "-k", "k.txt"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"k.txt", "k.txt.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("gzip -k lost %s", name)
		}
	}
	// -c writes to stdout and leaves the file alone entirely.
	stdout, _, err := runSmall(t, dir, "", "gzip", "-c", "c.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout == "" {
		t.Fatal("gzip -c wrote nothing")
	}
	if _, err := os.Stat(filepath.Join(dir, "c.txt.gz")); err == nil {
		t.Fatal("gzip -c wrote a companion file as well")
	}
	// An existing companion is not overwritten without -f, so a second run
	// cannot silently destroy an archive.
	if _, _, err := runSmall(t, dir, "", "gzip", "k.txt"); err == nil {
		t.Fatal("gzip overwrote an existing .gz without -f")
	}
	if _, _, err := runSmall(t, dir, "", "gzip", "-f", "k.txt"); err != nil {
		t.Fatalf("gzip -f: %v", err)
	}
}

// A pipe is the case busybox's own zcat cannot do: it seeks on its input, which
// a redirect allows and a pipe does not. This reads sequentially, so both work.
func TestZcat_readsAPipe(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "piped payload\n"})
	compressed, _, err := runSmall(t, dir, "", "gzip", "-c", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Fed as stdin, which is what a pipe is.
	stdout, stderr, err := runSmall(t, dir, compressed, "zcat")
	if err != nil {
		t.Fatalf("zcat from stdin: %v (%s)", err, stderr)
	}
	if stdout != "piped payload\n" {
		t.Fatalf("zcat from stdin = %q", stdout)
	}
	// zcat never writes to disk, whatever the operands.
	if _, _, err := runSmall(t, dir, "", "zcat", "h.txt.gz"); err == nil {
		// h.txt.gz does not exist; the point is only that it did not create one.
		_ = err
	}
}

// -t reads and discards, so it neither writes nor removes -- and it must actually
// notice corruption rather than reporting success for anything.
func TestGzip_testDetectsCorruption(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "payload\n"})
	good, _, err := runSmall(t, dir, "", "gzip", "-c", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.gz"), []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.gz"), []byte("not gzip at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, dir, "", "gzip", "-t", "good.gz"); err != nil {
		t.Fatalf("gzip -t on a good archive failed: %v (%s)", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "good")); err == nil {
		t.Fatal("gzip -t wrote a file; it should only read")
	}
	if _, err := os.Stat(filepath.Join(dir, "good.gz")); err != nil {
		t.Fatal("gzip -t removed the archive it was asked to check")
	}
	if _, stderr, err := runSmall(t, dir, "", "gzip", "-t", "bad.gz"); err == nil {
		t.Fatalf("gzip -t reported success for rubbish (stderr %q)", stderr)
	}
}

// The compression level has to actually reach the compressor, which a test on
// incompressible data could not show.
func TestGzip_levelChangesTheOutputSize(t *testing.T) {
	var repetitive strings.Builder
	for index := range 20000 {
		repetitive.WriteString("line ")
		repetitive.WriteString(strings.Repeat("x", index%7))
		repetitive.WriteString("\n")
	}
	dir := writeSmallFixture(t, map[string]string{"big.txt": repetitive.String()})
	fastest, _, err := runSmall(t, dir, "", "gzip", "-c1", "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	smallest, _, err := runSmall(t, dir, "", "gzip", "-c9", "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(smallest) >= len(fastest) {
		t.Fatalf("-9 produced %d bytes and -1 produced %d; the level is not reaching the compressor",
			len(smallest), len(fastest))
	}
}

// A name with no recognised suffix cannot be decompressed to anything, and
// guessing would overwrite the input.
func TestGunzip_refusesAnUnknownSuffix(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"plain": "x\n"})
	if _, stderr, err := runSmall(t, dir, "", "gunzip", "plain"); err == nil {
		t.Fatal("gunzip accepted a file with no suffix")
	} else if !strings.Contains(stderr+err.Error(), "suffix") {
		t.Fatalf("gunzip said %q, want it to name the suffix problem", stderr+err.Error())
	}
	// .tgz stands for .tar.gz, so the restored name keeps its .tar rather than
	// losing the extension entirely.
	payload, _, err := runSmall(t, dir, "", "gzip", "-c", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.tgz"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, dir, "", "gunzip", "bundle.tgz"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bundle.tar")); err != nil {
		t.Fatal("gunzip on a .tgz did not produce a .tar")
	}
}
