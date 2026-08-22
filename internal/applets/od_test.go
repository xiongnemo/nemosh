package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// od, hexdump, hd, uuencode and uudecode.
//
// 21 of 21 measured forms agree with busybox byte for byte, over "hello", an
// 84-character line and 300 random bytes. These tests pin the parts that are
// easy to get wrong.

// The word forms read each byte pair little-endian, which is the single most
// surprising thing about either tool: `he` is 0x68 0x65 and prints as 6568.
func TestOd_readsWordsLittleEndian(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello\n"})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		// 0o62550 is 0x6568, the pair "he" read low byte first.
		{applet: "od", args: []string{"h.txt"}, want: "0000000 062550 066154 005157\n0000006\n"},
		{applet: "hexdump", args: []string{"h.txt"}, want: "0000000 6568 6c6c 0a6f                         \n0000006\n"},
		{applet: "hd", args: []string{"h.txt"}, want: "00000000  68 65 6c 6c 6f 0a                                 |hello.|\n00000006\n"},
	} {
		got, stderr, err := runSmall(t, dir, "", test.applet, test.args...)
		if err != nil {
			t.Fatalf("%s: %v (%s)", test.applet, err, stderr)
		}
		if got != test.want {
			t.Fatalf("%s = %q\n  want %q", test.applet, got, test.want)
		}
	}
}

// hexdump pads a short line out to eight slots so a long dump's columns stay
// aligned; od leaves it short. Trimming trailing space, which is the obvious
// tidy-up, broke hexdump and passed od.
func TestOd_hexdumpPadsAndOdDoesNot(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello\n"})
	padded, _, err := runSmall(t, dir, "", "hexdump", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	firstLine := strings.Split(padded, "\n")[0]
	if !strings.HasSuffix(firstLine, "     ") {
		t.Fatalf("hexdump did not pad its short line: %q", firstLine)
	}
	short, _, err := runSmall(t, dir, "", "od", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(strings.Split(short, "\n")[0], " ") {
		t.Fatalf("od padded its short line: %q", strings.Split(short, "\n")[0])
	}
}

// The address field carries no trailing separator, because the character form's
// field is four wide including it. An extra space here put one before every
// `od -c` line.
func TestOd_characterForm(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"h.txt":   "hello\n",
		"esc.txt": "a\tb\n",
	})
	got, _, err := runSmall(t, dir, "", "od", "-c", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "0000000   h   e   l   l   o  \\n\n0000006\n"; got != want {
		t.Fatalf("od -c = %q\n  want %q", got, want)
	}
	// Escapes get their mnemonic; anything else unprintable gets octal.
	got, _, err = runSmall(t, dir, "", "od", "-c", "esc.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `\t`) {
		t.Fatalf("od -c did not name the tab: %q", got)
	}
	// -An drops the address entirely, which is what makes `od -An -tx1` a bare
	// hex stream.
	got, _, err = runSmall(t, dir, "", "od", "-An", "-tx1", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != " 68 65 6c 6c 6f 0a\n\n" && got != " 68 65 6c 6c 6f 0a\n" {
		t.Fatalf("od -An -tx1 = %q", got)
	}
	if _, _, err := runSmall(t, dir, "", "od", "-A", "q", "h.txt"); err == nil {
		t.Fatal("od -A q was accepted")
	}
}

func TestUuencode_roundTrips(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello\n"})
	encoded, stderr, err := runSmall(t, dir, "hello\n", "uuencode", "name.txt")
	if err != nil {
		t.Fatalf("uuencode: %v (%s)", err, stderr)
	}
	// The header names where uudecode will write, and the body ends with a lone
	// backtick -- a zero-length line, spelled with a backtick rather than a space
	// because trailing spaces do not survive mail.
	for _, want := range []string{"begin 644 name.txt\n", "\n`\nend\n"} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("uuencode output %q is missing %q", encoded, want)
		}
	}
	decoded, _, err := runSmall(t, dir, encoded, "uudecode", "-o", "-")
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello\n" {
		t.Fatalf("round trip = %q", decoded)
	}

	// Binary survives, which is the whole purpose.
	binary := string([]byte{0x00, 0xff, 0x80, 0x7f, 0x0a, 0x0d})
	encoded, _, err = runSmall(t, dir, binary, "uuencode", "b.bin")
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err = runSmall(t, dir, encoded, "uudecode", "-o", "-")
	if err != nil {
		t.Fatal(err)
	}
	if decoded != binary {
		t.Fatalf("binary round trip changed the bytes: %v became %v", []byte(binary), []byte(decoded))
	}

	// A long input crosses the 45-byte line boundary, which is where an
	// off-by-one in the length character would show.
	long := strings.Repeat("abcdefghij", 12)
	encoded, _, err = runSmall(t, dir, long, "uuencode", "l.txt")
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err = runSmall(t, dir, encoded, "uudecode", "-o", "-")
	if err != nil {
		t.Fatal(err)
	}
	if decoded != long {
		t.Fatalf("a multi-line round trip lost data: %d bytes became %d", len(long), len(decoded))
	}
}

// The name in the header came from the sender, so it is checked the way an
// archive entry is: `begin 644 ../../evil` is the same attack a tar entry would
// be.
func TestUudecode_refusesAnEscapingName(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	hostile := "begin 644 ../escape.txt\n!80``\n`\nend\n"
	if _, _, err := runSmall(t, root, hostile, "uudecode"); err == nil {
		t.Fatal("uudecode wrote to a name that escapes the directory")
	}
	entries, err := os.ReadDir(outer)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Join(outer, entry.Name()) != root {
			t.Fatalf("uudecode created %s outside the working directory", entry.Name())
		}
	}
	// A reserved device name is refused for the same reason.
	if _, _, err := runSmall(t, root, "begin 644 NUL\n!80``\n`\nend\n", "uudecode"); err == nil {
		t.Fatal("uudecode wrote to a reserved device name")
	}
	// And data with no header at all is refused rather than silently producing
	// nothing.
	if _, _, err := runSmall(t, root, "not uuencoded\n", "uudecode"); err == nil {
		t.Fatal("uudecode accepted input with no begin line")
	}
}

// uudecode writes to the name the *sender* chose unless -o says otherwise, so
// `-o -` is how the bytes are piped somewhere instead.
func TestUudecode_writesTheNamedFile(t *testing.T) {
	dir := t.TempDir()
	encoded, _, err := runSmall(t, dir, "payload\n", "uuencode", "out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runSmall(t, dir, encoded, "uudecode"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("uudecode did not write the named file: %v", err)
	}
	if string(data) != "payload\n" {
		t.Fatalf("the decoded file holds %q", data)
	}
}
