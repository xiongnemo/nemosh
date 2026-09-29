package applets

import "encoding/binary"

// executableHeader is busybox-w32's has_exec_format (win32/mingw.c:487-562) over a file's first
// bytes, of which it reads 1024: a #! script, an Actually Portable Executable, or a PE image that
// is a program rather than a DLL, for a subsystem that runs, native, a window, a console or
// POSIX's. Fewer than four bytes are no program.
func executableHeader(head []byte) bool {
	if len(head) < 4 {
		return false
	}
	if head[0] == '#' && head[1] == '!' {
		return true
	}
	if head[0] != 'M' || head[1] != 'Z' {
		return false
	}
	if len(head) > 6 && string(head[2:6]) == "qFpD" {
		return true
	}
	if len(head) <= 0x3f || binary.LittleEndian.Uint16(head[0x18:]) <= 0x3f {
		return false
	}
	// The PE header must be inside what was read, as busybox's 1024 bytes less 100 have it; a
	// shorter file busybox reads past the end of, where this says no.
	offset := int(binary.LittleEndian.Uint32(head[0x3c:]))
	if offset >= 1024-100 || offset+93 > len(head) || string(head[offset:offset+4]) != "PE\x00\x00" {
		return false
	}
	if magic := binary.LittleEndian.Uint16(head[offset+24:]); magic != 0x10b && magic != 0x20b {
		return false
	}
	// IMAGE_FILE_DLL among the characteristics.
	if binary.LittleEndian.Uint16(head[offset+22:])&0x2000 != 0 {
		return false
	}
	switch head[offset+92] {
	case 1, 2, 3, 7:
		return true
	}
	return false
}
