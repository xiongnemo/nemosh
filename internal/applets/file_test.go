package applets_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// file says what a file is in libmagic's words, file 5.44's as Debian has it: the text classes
// ascmagic names, with their line terminators; a script by its #!; a Windows executable as the
// msdos magic describes it; the first words for an ELF, the archives, the compressions and the
// images; -b, -i, --mime-type, -L and -N; and the names aligned after their colons. There was no
// file.
func TestFile_saysWhatLibmagicSays(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ascii", []byte("hello\nworld\n"))
	write("crlf", []byte("a\r\nb\r\n"))
	write("mixed", []byte("a\r\nb\n"))
	write("noeol", []byte("abc"))
	write("utf8", []byte("caf\xc3\xa9\n"))
	write("bom", []byte("\xef\xbb\xbfhi\n"))
	write("utf16", []byte("\xff\xfeh\x00i\x00\n\x00"))
	write("latin1", []byte("caf\xe9\n"))
	write("empty", nil)
	write("sh", []byte("#!/bin/sh\necho hi\n"))
	write("bash", []byte("#!/usr/bin/env bash\necho hi\n"))
	write("py", []byte("#!/usr/bin/python3\nprint(1)\n"))
	write("long", []byte(strings.Repeat("x", 400)+"\n"))
	write("esc", []byte("\x1b[31mred\x1b[0m\n"))
	write("data", []byte{0, 1, 2, 3, 0xff, 0xfe, 0})
	write("gz", []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 3, 0x4b, 0x4c, 0, 0, 0x55, 0x55, 0x55, 0x55, 6, 0, 0, 0})
	write("bz2", []byte("BZh91AY&SY"))
	write("xz", []byte("\xfd7zXZ\x00\x00\x04"))
	write("zip", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"))
	write("png", append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"), 0, 0, 2, 0x80, 0, 0, 1, 0xe0, 8, 6, 0, 0, 0))
	write("gif", []byte("GIF89a\x40\x01\xf0\x00"))
	write("pdf", []byte("%PDF-1.7\n%..."))
	write("sqlite", []byte("SQLite format 3\x00"))
	write("exe", fileTestPE(0x8664, 0x20b, 3, 0x0022|0x0200, 6))
	write("dll", fileTestPE(0x14c, 0x10b, 2, 0x2102, 4))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	elf := make([]byte, 64)
	copy(elf, "\x7fELF\x02\x01\x01\x00")
	binary.LittleEndian.PutUint16(elf[16:], 2)    // ET_EXEC
	binary.LittleEndian.PutUint16(elf[18:], 0x3e) // x86-64
	write("elf", elf)

	for _, test := range []struct{ name, want string }{
		{"ascii", "ASCII text"},
		{"crlf", "ASCII text, with CRLF line terminators"},
		{"mixed", "ASCII text, with CRLF, LF line terminators"},
		{"noeol", "ASCII text, with no line terminators"},
		{"utf8", "Unicode text, UTF-8 text"},
		{"bom", "Unicode text, UTF-8 (with BOM) text"},
		{"utf16", "Unicode text, UTF-16, little-endian text"},
		{"latin1", "ISO-8859 text"},
		{"empty", "empty"},
		{"sh", "POSIX shell script, ASCII text executable"},
		{"bash", "Bourne-Again shell script, ASCII text executable"},
		{"py", "Python script, ASCII text executable"},
		{"long", "ASCII text, with very long lines (400)"},
		{"esc", "ASCII text, with escape sequences"},
		{"data", "data"},
		{"gz", "gzip compressed data, from Unix, original size modulo 2^32 6"},
		{"bz2", "bzip2 compressed data, block size = 900k"},
		{"xz", "XZ compressed data, checksum CRC64"},
		{"zip", "Zip archive data, at least v2.0 to extract, compression method=deflate"},
		{"png", "PNG image data, 640 x 480, 8-bit/color RGBA, non-interlaced"},
		{"gif", "GIF image data, version 89a, 320 x 240"},
		{"pdf", "PDF document, version 1.7"},
		{"sqlite", "SQLite 3.x database"},
		{"exe", "PE32+ executable (console) x86-64 (stripped to external PDB), for MS Windows, 6 sections"},
		{"dll", "PE32 executable (DLL) (GUI) Intel 80386, for MS Windows, 4 sections"},
		{"elf", "ELF 64-bit LSB executable, x86-64, version 1 (SYSV)"},
		{"sub", "directory"},
	} {
		if stdout, stderr, err := runSmall(t, dir, "", "file", "-b", test.name); err != nil || stdout != test.want+"\n" {
			t.Errorf("file -b %s = %q, %v (%s); want %q", test.name, stdout, err, stderr, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"ascii", "pdf"}, "ascii: ASCII text\npdf:   PDF document, version 1.7\n"},
		{[]string{"-N", "ascii", "pdf"}, "ascii: ASCII text\npdf: PDF document, version 1.7\n"},
		{[]string{"-i", "ascii", "utf8", "exe"}, "ascii: text/plain; charset=us-ascii\nutf8:  text/plain; charset=utf-8\nexe:   application/x-dosexec; charset=binary\n"},
		{[]string{"--mime-type", "-b", "sh", "png", "empty", "sub"}, "text/x-shellscript\nimage/png\ninode/x-empty\ninode/directory\n"},
		{[]string{"nope"}, "nope: cannot open `nope' (No such file or directory)\n"},
	} {
		if stdout, stderr, err := runSmall(t, dir, "", "file", test.args...); err != nil || stdout != test.want {
			t.Errorf("file %q = %q, %v (%s); want %q", test.args, stdout, err, stderr, test.want)
		}
	}
}

// A real executable, the one running this test, is a PE32 or PE32+ console program for MS Windows,
// of the machine it was built for.
func TestFile_describesARealExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	stdout, _, err := runSmall(t, filepath.Dir(self), "", "file", "-b", filepath.Base(self))
	machine := map[string]string{"amd64": "x86-64", "386": "Intel 80386", "arm64": "Aarch64"}[runtime.GOARCH]
	if err != nil || !strings.HasPrefix(stdout, "PE32") || !strings.Contains(stdout, "(console) "+machine) || !strings.Contains(stdout, ", for MS Windows, ") {
		t.Errorf("file -b of this test = %q, %v; want a %s console PE", stdout, err, machine)
	}
	t.Logf("file -b %s: %s", filepath.Base(self), stdout)
}

// fileTestPE is the smallest PE header the msdos magic reads: MZ, e_lfanew, PE, the machine, the
// sections, the characteristics, the optional header's magic and its subsystem.
func fileTestPE(machine, magic, subsystem, characteristics, sections uint16) []byte {
	image := make([]byte, 0x200)
	copy(image, "MZ")
	binary.LittleEndian.PutUint32(image[0x3c:], 0x80)
	copy(image[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(image[0x84:], machine)
	binary.LittleEndian.PutUint16(image[0x86:], sections)
	binary.LittleEndian.PutUint16(image[0x96:], characteristics)
	binary.LittleEndian.PutUint16(image[0x98:], magic)
	binary.LittleEndian.PutUint16(image[0x98+68:], subsystem)
	return image
}
