package applets

import (
	"encoding/binary"
	"testing"
)

// peHeader is the first 1024 bytes of a PE image with the given characteristics, optional header
// magic and subsystem, its header at 0x80.
func peHeader(characteristics, magic uint16, subsystem byte) []byte {
	head := make([]byte, 1024)
	copy(head, "MZ")
	binary.LittleEndian.PutUint16(head[0x18:], 0x40)
	binary.LittleEndian.PutUint32(head[0x3c:], 0x80)
	copy(head[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(head[0x80+22:], characteristics)
	binary.LittleEndian.PutUint16(head[0x80+24:], magic)
	head[0x80+92] = subsystem
	return head
}

// A file is a program to busybox-w32 when it begins #!, is an Actually Portable Executable, or
// is a PE image that is no DLL, for a subsystem that runs.
func TestExecutableHeader_isBusyboxW32s(t *testing.T) {
	far := peHeader(0x0102, 0x10b, 3)
	binary.LittleEndian.PutUint32(far[0x3c:], 924)
	for _, test := range []struct {
		name string
		head []byte
		want bool
	}{
		{"a script", []byte("#!/bin/sh\n"), true},
		{"too short to judge", []byte("#!"), false},
		{"an APE", []byte("MZqFpD='\n"), true},
		{"a console program", peHeader(0x0102, 0x10b, 3), true},
		{"a 64-bit window program", peHeader(0x0022, 0x20b, 2), true},
		{"a native one", peHeader(0x0102, 0x10b, 1), true},
		{"a POSIX console one", peHeader(0x0102, 0x10b, 7), true},
		{"a DLL", peHeader(0x2102, 0x10b, 3), false},
		{"an EFI application", peHeader(0x0102, 0x20b, 10), false},
		{"a ROM image", peHeader(0x0102, 0x107, 3), false},
		{"MZ and no PE header", append([]byte("MZ"), make([]byte, 100)...), false},
		{"a PE header too far in", far, false},
		{"ELF", []byte("\x7fELF\x02\x01\x01"), false},
		{"text", []byte("plain text\n"), false},
	} {
		if got := executableHeader(test.head); got != test.want {
			t.Errorf("%s: executableHeader = %v, want %v", test.name, got, test.want)
		}
	}
}
