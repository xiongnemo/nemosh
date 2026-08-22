package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dos2unix, unix2dos and iconv.
//
// The three encoding tools, and the reason iconv is here: `docs/support-matrix.md`
// recorded `sed -i` and `wc -m` as outstanding on UTF-16 input because neither had
// a policy for which encoding to *write*. iconv is the tool whose whole job is
// that choice, so the policy is settled here and pinned by these tests.

func TestDos2unix_convertsInPlace(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"crlf.txt": "a\r\nb\r\n",
		"lf.txt":   "a\nb\n",
	})
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	// A file operand is converted in place and *nothing* goes to stdout. That is
	// the trap in this applet's default and it is what busybox does.
	stdout, stderr, err := runSmall(t, dir, "", "dos2unix", "crlf.txt")
	if err != nil {
		t.Fatalf("dos2unix: %v (%s)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("dos2unix wrote %q to stdout; it converts in place", stdout)
	}
	if got := read("crlf.txt"); got != "a\nb\n" {
		t.Fatalf("dos2unix left %q", got)
	}

	// With no operand it is a filter.
	stdout, _, err = runSmall(t, dir, "x\r\ny\r\n", "dos2unix")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "x\ny\n" {
		t.Fatalf("dos2unix as a filter = %q", stdout)
	}

	// unix2dos is the same implementation the other way round.
	if _, _, err := runSmall(t, dir, "", "unix2dos", "lf.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("lf.txt"); got != "a\r\nb\r\n" {
		t.Fatalf("unix2dos left %q", got)
	}

	// Running unix2dos twice must not produce CRCRLF. The input is normalised to
	// LF first, which is what makes it idempotent -- a bare LF-to-CRLF
	// replacement doubles the CR on its second run.
	if _, _, err := runSmall(t, dir, "", "unix2dos", "lf.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("lf.txt"); got != "a\r\nb\r\n" {
		t.Fatalf("unix2dos twice left %q, want it unchanged", got)
	}

	// -u and -d name the direction outright, so the name is only a default.
	if _, _, err := runSmall(t, dir, "", "unix2dos", "-u", "lf.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("lf.txt"); got != "a\nb\n" {
		t.Fatalf("unix2dos -u left %q, want Unix endings", got)
	}
}

// A lone carriage return is not a CRLF and must survive: on an old Mac file it is
// the line ending itself, and dropping it would join every line into one.
func TestDos2unix_leavesALoneCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runSmall(t, dir, "a\rb\r", "dos2unix")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "a\rb\r" {
		t.Fatalf("dos2unix changed a lone CR: %q", stdout)
	}
	// And binary data with a stray CR is not rewritten either.
	stdout, _, err = runSmall(t, dir, "\x00\r\x01", "dos2unix")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "\x00\r\x01" {
		t.Fatalf("dos2unix touched binary data: %q", stdout)
	}
}

// An unchanged file is not rewritten, so its modification time survives and a
// build system is not told it changed.
func TestDos2unix_doesNotRewriteAnUnchangedFile(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"already.txt": "a\nb\n"})
	path := filepath.Join(dir, "already.txt")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, dir, "", "dos2unix", "already.txt"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("dos2unix rewrote a file that needed no conversion")
	}
}

func TestIconv_convertsBetweenNamedEncodings(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"u8.txt": "héllo\n"})

	// Both ends default to UTF-8, so a bare run is a no-op rather than a
	// surprise.
	stdout, stderr, err := runSmall(t, dir, "", "iconv", "u8.txt")
	if err != nil {
		t.Fatalf("iconv: %v (%s)", err, stderr)
	}
	if stdout != "héllo\n" {
		t.Fatalf("iconv with no options = %q, want the input unchanged", stdout)
	}

	// UTF-8 to Latin-1: é becomes one byte, 0xe9.
	stdout, _, err = runSmall(t, dir, "", "iconv", "-f", "UTF-8", "-t", "ISO-8859-1", "u8.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "h\xe9llo\n" {
		t.Fatalf("iconv to Latin-1 = %q, want a single 0xe9", stdout)
	}

	// And back again, which is the round trip that matters.
	back, _, err := runSmall(t, dir, stdout, "iconv", "-f", "ISO-8859-1", "-t", "UTF-8")
	if err != nil {
		t.Fatal(err)
	}
	if back != "héllo\n" {
		t.Fatalf("Latin-1 round trip = %q", back)
	}

	// UTF-16 in both byte orders, with no mark added: the explicit LE and BE
	// names mean the caller has already decided, and an uninvited BOM breaks
	// `#!` lines and CSV headers.
	for _, name := range []string{"UTF-16LE", "UTF-16BE"} {
		encoded, _, err := runSmall(t, dir, "", "iconv", "-f", "UTF-8", "-t", name, "u8.txt")
		if err != nil {
			t.Fatalf("iconv to %s: %v", name, err)
		}
		if strings.HasPrefix(encoded, "\xff\xfe") || strings.HasPrefix(encoded, "\xfe\xff") {
			t.Fatalf("iconv to %s added a byte-order mark: %q", name, encoded[:2])
		}
		decoded, _, err := runSmall(t, dir, encoded, "iconv", "-f", name, "-t", "UTF-8")
		if err != nil {
			t.Fatal(err)
		}
		if decoded != "héllo\n" {
			t.Fatalf("%s round trip = %q", name, decoded)
		}
	}

	// The code pages a Windows file is actually likely to be in.
	for _, name := range []string{"GBK", "Shift_JIS", "windows-1252", "EUC-KR", "Big5"} {
		if _, _, err := runSmall(t, dir, "", "iconv", "-f", "UTF-8", "-t", name, "u8.txt"); err != nil {
			// windows-1252 holds é; the CJK sets may not, and that is a loud
			// failure by design rather than a silent substitution.
			if name == "windows-1252" {
				t.Fatalf("iconv to %s failed: %v", name, err)
			}
		}
	}
}

// A character the target cannot represent is an error, not a silent
// substitution -- silently losing characters is how text gets corrupted. -c is
// how a caller asks for the lossy behaviour explicitly.
func TestIconv_refusesWhatItCannotRepresentUnlessAsked(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runSmall(t, dir, "中\n", "iconv", "-f", "UTF-8", "-t", "ISO-8859-1"); err == nil {
		t.Fatal("iconv silently converted a character Latin-1 cannot hold")
	}
	stdout, _, err := runSmall(t, dir, "a中b\n", "iconv", "-c", "-f", "UTF-8", "-t", "ISO-8859-1")
	if err != nil {
		t.Fatalf("iconv -c: %v", err)
	}
	if !strings.Contains(stdout, "a") || !strings.Contains(stdout, "b") {
		t.Fatalf("iconv -c dropped too much: %q", stdout)
	}
	// An encoding nobody has is refused by name rather than guessed at.
	if _, stderr, err := runSmall(t, dir, "x", "iconv", "-f", "NOSUCH-8"); err == nil {
		t.Fatal("iconv accepted an unknown encoding")
	} else if !strings.Contains(stderr+err.Error(), "NOSUCH-8") {
		t.Fatalf("iconv said %q, want it to name the encoding", stderr+err.Error())
	}
}

func TestIconv_listsWhatItKnows(t *testing.T) {
	stdout, _, err := runSmall(t, t.TempDir(), "", "iconv", "-l")
	if err != nil {
		t.Fatal(err)
	}
	// Every name listed must actually work, or a name that appears and then
	// fails would be worse than one that never appeared.
	for _, name := range strings.Fields(stdout) {
		if _, _, err := runSmall(t, t.TempDir(), "abc\n", "iconv", "-f", "UTF-8", "-t", name); err != nil {
			t.Fatalf("iconv -l offers %q but converting to it failed: %v", name, err)
		}
	}
	for _, want := range []string{"UTF-8", "UTF-16LE", "GBK", "Shift_JIS"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("iconv -l is missing %q", want)
		}
	}
}
