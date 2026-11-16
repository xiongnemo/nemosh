package applets_test

import (
	"strings"
	"testing"
	"unicode/utf16"
)

// iconv is busybox-w32's, Windows's own conversion a character at a time: each answer here is
// busybox-w32 FRP-6354's, measured, but for the input split where a read ends, which
// busybox-w32 drops a character at under -c.
func TestIconv_convertsAsBusyboxDoes(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	for _, test := range []struct {
		name, stdin, stdout, err string
		args                     []string
	}{
		{name: "nothing named is UTF-8 to UTF-8", stdin: "a中b\n", stdout: "a中b\n"},
		{name: "a character Latin-1 has not got", stdin: "a中b\n", args: []string{"-f", "UTF-8", "-t", "ISO-8859-1"},
			stdout: "a", err: "conversion error: Illegal byte sequence"},
		{name: "-c passes over it", stdin: "a中b\n", args: []string{"-c", "-f", "UTF-8", "-t", "ISO-8859-1"}, stdout: "ab\n"},
		{name: "input that is no character", stdin: "a\xffb\n", args: []string{"-f", "UTF-8", "-t", "UTF-16LE"},
			stdout: "a\x00", err: "conversion error: Illegal byte sequence"},
		{name: "-c passes over that too", stdin: "a\xffb\n", args: []string{"-c", "-f", "UTF-8", "-t", "UTF-16LE"}, stdout: "a\x00b\x00\n\x00"},
		{name: "UTF-16 is written with a mark", stdin: "a中b\n", args: []string{"-t", "UTF-16"}, stdout: "\xff\xfea\x00\x2d\x4eb\x00\n\x00"},
		{name: "UTF-16LE without one", stdin: "a中\n", args: []string{"-t", "UTF-16LE"}, stdout: "a\x00\x2d\x4e\n\x00"},
		{name: "UTF-16 read by its mark", stdin: "\xfe\xff\x00a\x4e\x2d", args: []string{"-f", "UTF-16"}, stdout: "a中"},
		{name: "UTF-32BE", stdin: "😀", args: []string{"-t", "UTF-32BE"}, stdout: "\x00\x01\xf6\x00"},
		{name: "Latin-1 by a code page number", stdin: "\xe9", args: []string{"-f", "CP28591"}, stdout: "é"},
		{name: "and by the number alone", stdin: "é", args: []string{"-t", "1252"}, stdout: "\xe9"},
		{name: "GBK", stdin: "中文", args: []string{"-t", "GBK"}, stdout: "\xd6\xd0\xce\xc4"},
		{name: "CP932's yen is its backslash", stdin: "¥", args: []string{"-t", "CP932"}, stdout: "\\"},
		{name: "no best fit for a character the target has not got", stdin: "ā", args: []string{"-t", "ISO-8859-1"},
			err: "conversion error: Illegal byte sequence"},
		{name: "ASCII refuses a byte past it", stdin: "a\xe9", args: []string{"-f", "ASCII"},
			stdout: "a", err: "conversion error: Illegal byte sequence"},
		{name: "a character the input stops inside", stdin: "a\xe4\xb8", args: []string{"-t", "UTF-16LE"},
			stdout: "a\x00", err: "conversion error: Invalid argument"},
		{name: "an encoding Windows has not got", stdin: "x", args: []string{"-f", "NOSUCH-8"}, err: "iconv_open error: Invalid argument"},
		{name: "nor a code page", stdin: "x", args: []string{"-t", "CP99999"}, err: "iconv_open error: Invalid argument"},
	} {
		stdout, _, err := runSmall(t, dir, test.stdin, "iconv", test.args...)
		got := ""
		if err != nil {
			got = err.Error()
		}
		if stdout != test.stdout || got != test.err {
			t.Errorf("%s: iconv %q of %q = %q, %q; want %q, %q", test.name, test.args, test.stdin, stdout, got, test.stdout, test.err)
		}
	}
}

// EUC-JP and ISO-2022-JP go through MLang, and come back as they went; ISO-2022-JP ends in
// ASCII, as it has to.
func TestIconv_japaneseThroughMLang(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	text := "日本語 and ｶﾅ\n"
	for _, name := range []string{"EUC-JP", "ISO-2022-JP", "CP50221", "SHIFT_JIS"} {
		encoded, _, err := runSmall(t, dir, text, "iconv", "-t", name)
		if err != nil || encoded == text {
			t.Fatalf("to %s: %q, %v", name, encoded, err)
		}
		if strings.HasPrefix(name, "ISO") && !strings.HasSuffix(encoded, "\x1b(B\n") {
			t.Errorf("to %s did not end in ASCII: %q", name, encoded)
		}
		back, _, err := runSmall(t, dir, encoded, "iconv", "-f", name)
		if err != nil || back != text {
			t.Errorf("%s and back: %q, %v; want %q", name, back, err, text)
		}
	}
	if encoded, _, _ := runSmall(t, dir, "日本", "iconv", "-t", "ISO-2022-JP"); encoded != "\x1b$BF|K\\\x1b(B" {
		t.Errorf("ISO-2022-JP of 日本 = %q", encoded)
	}
}

// A character split where one read ends and the next begins is converted once, whole: three
// bytes of UTF-8 and four of a UTF-16 surrogate pair, past 32 KiB.
func TestIconv_aCharacterSplitBetweenReads(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{})
	text := "a" + strings.Repeat("中", 20000) + strings.Repeat("😀", 10000)
	var want strings.Builder
	for _, unit := range utf16.Encode([]rune(text)) {
		want.WriteByte(byte(unit))
		want.WriteByte(byte(unit >> 8))
	}
	encoded, _, err := runSmall(t, dir, text, "iconv", "-t", "UTF-16LE")
	if err != nil || encoded != want.String() {
		t.Fatalf("UTF-16LE of %d bytes: %d bytes, %v; want %d", len(text), len(encoded), err, want.Len())
	}
	back, _, err := runSmall(t, dir, encoded, "iconv", "-f", "UTF-16LE")
	if err != nil || back != text {
		t.Fatalf("and back: %d bytes, %v", len(back), err)
	}
	dropped, _, err := runSmall(t, dir, text, "iconv", "-c", "-t", "UTF-16LE")
	if err != nil || dropped != want.String() {
		t.Fatalf("with -c: %d bytes, %v; want %d", len(dropped), err, want.Len())
	}
	// Thousands passed over, each where it stood, past where a read ends.
	latin, _, err := runSmall(t, dir, strings.Repeat("a中é😀b\n", 3000), "iconv", "-c", "-t", "ISO-8859-1")
	if want := strings.Repeat("a\xe9b\n", 3000); err != nil || latin != want {
		t.Fatalf("Latin-1 with -c: %d bytes, %v; want %d", len(latin), err, len(want))
	}
}

// -l is busybox-w32's 358 aliases, in its order.
func TestIconv_listsBusyboxsAliases(t *testing.T) {
	stdout, _, err := runSmall(t, t.TempDir(), "", "iconv", "-l")
	names := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if err != nil || len(names) != 358 || names[0] != "CP65001" || names[1] != "UTF8" || names[357] == "" {
		t.Fatalf("iconv -l gave %d names beginning %q, %v", len(names), names[:min(3, len(names))], err)
	}
}
