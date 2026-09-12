package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `sed -i` and `wc -m` over UTF-16, the two items docs/support-matrix.md carried as
// outstanding.
//
// Both were blocked on the same question -- which encoding to *write* -- and `iconv`
// answered it on 2026-08-22: an encoding is named, never guessed. Here the name is the
// file's own byte-order mark, so re-encoding to it is not a guess either.
//
// This matters more on Windows than the deferral suggested. Windows PowerShell 5.1's `>`
// writes UTF-16LE, so these are not exotic files on this platform; they are what half the
// tooling produces.

func writeBytes(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The encoder and decoder agree, over the awkward cases: an astral character that is a
// surrogate pair, a CJK character, and both byte orders.
func TestTextEncoding_roundTrips(t *testing.T) {
	for _, text := range []string{
		"hello\n",
		"路径 is a path\n",
		"an emoji \U0001F389 and text\n",
		"",
		"no trailing newline",
		"\n\n\n",
	} {
		for _, bigEndian := range []bool{false, true} {
			encoding := encodingUTF16LE
			if bigEndian {
				encoding = encodingUTF16BE
			}
			raw := utf16Bytes(text, bigEndian, true)
			if got := detectTextEncoding(raw); got != encoding {
				t.Fatalf("%q: detected %v, want %v", text, got, encoding)
			}
			decoded := readAllString(t, decodeTextInput(strings.NewReader(string(raw))))
			if decoded != text {
				t.Fatalf("decoding %q gave %q", text, decoded)
			}
			if again := encoding.encode([]byte(decoded)); string(again) != string(raw) {
				t.Fatalf("%q did not round trip:\n  got  % x\n  want % x", text, again, raw)
			}
		}
	}
	// A file with no mark is not text this code claims to understand, so both
	// directions are the identity -- which is what keeps the byte-exact path exact
	// rather than merely equivalent.
	plain := []byte{0x00, 0x01, 0xFF, 'a', '\n'}
	if got := detectTextEncoding(plain); got != encodingBytes {
		t.Fatalf("bytes with no mark detected as %v", got)
	}
	if got := encodingBytes.encode(plain); string(got) != string(plain) {
		t.Fatalf("encodingBytes changed its input: % x", got)
	}
}

func readAllString(t *testing.T, reader interface{ Read([]byte) (int, error) }) string {
	t.Helper()
	var out strings.Builder
	buffer := make([]byte, 64)
	for {
		read, err := reader.Read(buffer)
		out.Write(buffer[:read])
		if err != nil {
			return out.String()
		}
	}
}

// `sed` matches a UTF-16 file now, where it used to match nothing and copy it through.
func TestSed_matchesUtf16AndPrintsUtf8(t *testing.T) {
	root := t.TempDir()
	writeBytes(t, root, "u16.txt", utf16Bytes("hello world\nsecond line\n", false, true))
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	var stdout, stderr strings.Builder
	if err := newSedApplet().Run(ctx, []string{"s/hello/goodbye/", "u16.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("sed: %v (%s)", err, stderr.String())
	}
	// Printed output is UTF-8, which is the rule grep already follows and the only one
	// that does not make every applet remember what it read.
	if got, want := stdout.String(), "goodbye world\nsecond line\n"; got != want {
		t.Fatalf("sed printed %q, want %q", got, want)
	}
}

// `sed -i` writes the file back in the encoding it arrived in, which is the half that was
// deferred. A Notepad file that came back as UTF-8 would be silently broken for every
// other tool that reads it.
func TestSed_inPlaceKeepsTheEncoding(t *testing.T) {
	for _, test := range []struct {
		name      string
		bigEndian bool
	}{
		{name: "little endian", bigEndian: false},
		{name: "big endian", bigEndian: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			original := utf16Bytes("hello world\n路径\n", test.bigEndian, true)
			path := writeBytes(t, root, "u16.txt", original)
			ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

			var stdout, stderr strings.Builder
			if err := newSedApplet().Run(ctx, []string{"-i", "s/hello/goodbye/", "u16.txt"},
				strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("sed -i: %v (%s)", err, stderr.String())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := utf16Bytes("goodbye world\n路径\n", test.bigEndian, true)
			if string(got) != string(want) {
				t.Fatalf("sed -i wrote\n  % x\nwant\n  % x", got, want)
			}
			// The mark survived, which is what Notepad needs to open it at all.
			if len(got) < 2 || got[0] == 'g' {
				t.Fatalf("the byte-order mark is gone: % x", got[:min(8, len(got))])
			}
		})
	}
}

// A no-op script round-trips byte for byte, including the mark -- the property that makes
// `sed -i` safe to run over a directory.
func TestSed_inPlaceRoundTripsUnchanged(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "utf16le.txt", data: utf16Bytes("line one\nline two\n", false, true)},
		{name: "utf16be.txt", data: utf16Bytes("line one\nline two\n", true, true)},
		{name: "utf8bom.txt", data: append([]byte{0xEF, 0xBB, 0xBF}, "hello\n"...)},
		{name: "plain.txt", data: []byte("hello\nworld\n")},
		// No trailing newline: the case that appended a byte until 2026-08-23.
		{name: "noeol.txt", data: utf16Bytes("no newline here", false, true)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeBytes(t, root, test.name, test.data)
			ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})
			var stdout, stderr strings.Builder
			if err := newSedApplet().Run(ctx, []string{"-i", "s/zzzznotthere/x/", test.name},
				strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("sed -i: %v (%s)", err, stderr.String())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(test.data) {
				t.Fatalf("a no-op sed -i changed the file:\n  got  % x\n  want % x", got, test.data)
			}
		})
	}
}

// `wc -m` counts characters and `-c` counts bytes, which on a UTF-16 file are different
// numbers over the same stream. That they had to be was the whole reason `-m` was
// outstanding.
func TestWc_charactersAndBytesDifferOnUtf16(t *testing.T) {
	root := t.TempDir()
	const text = "hello\n路径\n"
	raw := utf16Bytes(text, false, true)
	writeBytes(t, root, "u16.txt", raw)
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	var stdout, stderr strings.Builder
	if err := newWcApplet().Run(ctx, []string{"-m", "-c", "-l", "-w", "u16.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("wc: %v (%s)", err, stderr.String())
	}
	fields := strings.Fields(stdout.String())
	if len(fields) != 5 {
		t.Fatalf("wc printed %q, want four counts and a name", stdout.String())
	}
	// Order is lines, words, chars, bytes -- the order printWcCounts uses.
	lines, words, chars, bytes := fields[0], fields[1], fields[2], fields[3]
	if want := itoa(len([]rune(text))); chars != want {
		t.Errorf("-m counted %s characters, want %s", chars, want)
	}
	if want := itoa(len(raw)); bytes != want {
		t.Errorf("-c counted %s bytes, want %s -- the bytes on disk", bytes, want)
	}
	if chars == bytes {
		t.Error("-m and -c agree on a UTF-16 file, which is the bug this closes")
	}
	// And the counts that only mean anything on decoded text are right too.
	if lines != "2" {
		t.Errorf("-l counted %s lines, want 2", lines)
	}
	// Two: `hello` and `路径`. Words are whitespace-separated, so the CJK pair is one
	// word however many characters it is -- which is the same answer both references
	// give and not the same as the character count.
	if words != "2" {
		t.Errorf("-w counted %s words, want 2", words)
	}
}

// An ASCII file is unaffected, which is the regression that matters: every existing
// script's `wc` output must be what it was.
func TestWc_plainTextIsUnchanged(t *testing.T) {
	root := t.TempDir()
	const text = "one two\nthree\n"
	writeBytes(t, root, "plain.txt", []byte(text))
	ctx := WithProcessView(t.Context(), hostProcessView{cwd: root})

	var stdout, stderr strings.Builder
	if err := newWcApplet().Run(ctx, []string{"-l", "-w", "-m", "-c", "plain.txt"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(stdout.String())
	if len(fields) != 5 {
		t.Fatalf("wc printed %q", stdout.String())
	}
	for index, want := range []string{"2", "3", itoa(len(text)), itoa(len(text))} {
		if fields[index] != want {
			t.Errorf("field %d is %s, want %s (from %q)", index, fields[index], want, stdout.String())
		}
	}
}
