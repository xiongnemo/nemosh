package applets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// The magic file knows binary files by, in libmagic's words: the msdos magic for a Windows
// executable in full, and for the rest the first words and the header fields named here. tail is
// a file's last four bytes, which a gzip file ends its original size with.

func describeMagic(head, tail []byte) (string, string, bool) {
	little := binary.LittleEndian
	has := func(offset int, magic string) bool {
		return len(head) >= offset+len(magic) && string(head[offset:offset+len(magic)]) == magic
	}
	switch {
	case has(0, "MZ"):
		return filePE(head), "application/x-dosexec", true
	case has(0, "\x7fELF") && len(head) >= 20:
		return fileELF(head)
	case has(0, "\x1f\x8b") && len(head) >= 10:
		return fileGzip(head, tail), "application/gzip", true
	case has(0, "BZh") && len(head) >= 4 && head[3] >= '1' && head[3] <= '9':
		return fmt.Sprintf("bzip2 compressed data, block size = %c00k", head[3]), "application/x-bzip2", true
	case has(0, "\xfd7zXZ\x00") && len(head) >= 8:
		checks := map[byte]string{0: "NONE", 1: "CRC32", 4: "CRC64", 10: "SHA-256"}
		return "XZ compressed data, checksum " + checks[head[7]&0x0f], "application/x-xz", true
	case has(0, "\x28\xb5\x2f\xfd"):
		return "Zstandard compressed data (v0.8+), Dictionary ID: None", "application/zstd", true
	case has(0, "PK\x05\x06"):
		return "Zip archive data (empty)", "application/zip", true
	case has(0, "PK\x03\x04") && len(head) >= 10:
		version, method := little.Uint16(head[4:]), little.Uint16(head[8:])
		methods := map[uint16]string{0: "store", 8: "deflate", 9: "deflate64", 12: "bzip2", 14: "lzma", 93: "zstd", 95: "xz", 99: "AES Encrypted"}
		name, known := methods[method]
		if !known {
			name = fmt.Sprintf("%#x", method)
		}
		return fmt.Sprintf("Zip archive data, at least v%d.%d to extract, compression method=%s", version/10, version%10, name), "application/zip", true
	case has(0, "7z\xbc\xaf\x27\x1c") && len(head) >= 8:
		return fmt.Sprintf("7-zip archive data, version %d.%d", head[6], head[7]), "application/x-7z-compressed", true
	case has(257, "ustar  \x00"):
		return "POSIX tar archive (GNU)", "application/x-tar", true
	case has(257, "ustar\x00"):
		return "POSIX tar archive", "application/x-tar", true
	case has(0, "\x89PNG\r\n\x1a\n") && len(head) >= 29:
		return filePNG(head), "image/png", true
	case (has(0, "GIF87a") || has(0, "GIF89a")) && len(head) >= 10:
		return fmt.Sprintf("GIF image data, version %s, %d x %d", head[3:6], little.Uint16(head[6:]), little.Uint16(head[8:])), "image/gif", true
	case has(0, "\xff\xd8\xff"):
		return "JPEG image data", "image/jpeg", true
	case has(0, "RIFF") && has(8, "WEBP"):
		return "RIFF (little-endian) data, Web/P image", "image/webp", true
	case has(0, "%PDF-"):
		version, _, _ := bytes.Cut(head[5:min(len(head), 16)], []byte("\n"))
		return "PDF document, version " + strings.TrimSpace(strings.Fields(string(version) + " ")[0]), "application/pdf", true
	case has(0, "SQLite format 3\x00"):
		return "SQLite 3.x database", "application/vnd.sqlite3", true
	}
	return "", "", false
}

// filePE is the msdos magic's description of a Windows executable: PE32 or PE32+, (DLL), the
// subsystem, the machine, (stripped to external PDB), Mono/.Net assembly, for MS Windows, and the
// sections when there is more than one.
func filePE(head []byte) string {
	little := binary.LittleEndian
	if len(head) < 0x40 {
		return "MS-DOS executable"
	}
	at := int(little.Uint32(head[0x3c:]))
	if at < 0 || at+24+92 > len(head) || string(head[at:at+4]) != "PE\x00\x00" {
		return "MS-DOS executable"
	}
	machine, sections, characteristics := little.Uint16(head[at+4:]), little.Uint16(head[at+6:]), little.Uint16(head[at+22:])
	magic, subsystem := little.Uint16(head[at+24:]), little.Uint16(head[at+92:])
	description := "PE32 executable"
	clr := at + 232
	if magic == 0x20b {
		description, clr = "PE32+ executable", at+248
	}
	if characteristics&0x2000 != 0 {
		description += " (DLL)"
	}
	subsystems := map[uint16]string{1: "native", 2: "GUI", 3: "console", 7: "POSIX CUI", 9: "Windows CE GUI", 10: "EFI application",
		11: "EFI boot service driver", 12: "EFI runtime driver", 13: "EFI ROM", 14: "XBOX", 16: "Windows boot application"}
	if name, known := subsystems[subsystem]; known {
		description += " (" + name + ")"
	} else {
		description += fmt.Sprintf(" (Unknown subsystem %#x)", subsystem)
	}
	machines := map[uint16]string{0x14c: "Intel 80386", 0x8664: "x86-64", 0xaa64: "Aarch64", 0x1c0: "ARM", 0x1c4: "ARMv7 Thumb",
		0x200: "Intel Itanium", 0x5064: "RISC-V 64-bit"}
	if name, known := machines[machine]; known {
		description += " " + name
	} else {
		description += fmt.Sprintf(" Unknown processor type %#x", machine)
	}
	if characteristics&0x0200 != 0 {
		description += " (stripped to external PDB)"
	}
	if clr+4 <= len(head) && little.Uint32(head[clr:]) > 0 {
		description += " Mono/.Net assembly"
	}
	description += ", for MS Windows"
	if sections > 1 {
		description += fmt.Sprintf(", %d sections", sections)
	}
	return description
}

// fileELF is an ELF's first fields: its class, its byte order, its type -- a pie executable for a
// shared object with an interpreter -- its machine, its version and its ABI.
func fileELF(head []byte) (string, string, bool) {
	var order binary.ByteOrder = binary.LittleEndian
	endian := "LSB"
	if head[5] == 2 {
		order, endian = binary.BigEndian, "MSB"
	}
	class := map[byte]string{1: "32-bit", 2: "64-bit"}[head[4]]
	types := map[uint16][2]string{1: {"relocatable", "application/x-object"}, 2: {"executable", "application/x-executable"},
		3: {"shared object", "application/x-sharedlib"}, 4: {"core file", "application/x-coredump"}}
	kind, known := types[order.Uint16(head[16:])]
	if !known {
		kind = [2]string{"unknown type", "application/octet-stream"}
	}
	if order.Uint16(head[16:]) == 3 && fileELFInterpreter(head, order) {
		kind = [2]string{"pie executable", "application/x-pie-executable"}
	}
	machines := map[uint16]string{3: "Intel 80386", 0x3e: "x86-64", 0xb7: "ARM aarch64", 0x28: "ARM", 0xf3: "UCB RISC-V", 8: "MIPS", 0x14: "PowerPC", 0x15: "64-bit PowerPC or cisco 7500"}
	machine, known := machines[order.Uint16(head[18:])]
	if !known {
		machine = fmt.Sprintf("*unknown arch %#x*", order.Uint16(head[18:]))
	}
	abis := map[byte]string{0: "SYSV", 3: "GNU/Linux", 6: "Solaris", 9: "FreeBSD", 12: "OpenBSD"}
	abi, known := abis[head[7]]
	if !known {
		abi = fmt.Sprintf("unknown OS ABI %d", head[7])
	}
	return fmt.Sprintf("ELF %s %s %s, %s, version %d (%s)", class, endian, kind[0], machine, head[6], abi), kind[1], true
}

// fileELFInterpreter is whether the program headers ask for an interpreter, PT_INTERP, which tells
// a position-independent executable from a library.
func fileELFInterpreter(head []byte, order binary.ByteOrder) bool {
	offset, size, count := 0, 0, 0
	if head[4] == 2 && len(head) >= 64 {
		offset, size, count = int(order.Uint64(head[32:])), int(order.Uint16(head[54:])), int(order.Uint16(head[56:]))
	} else if len(head) >= 52 {
		offset, size, count = int(order.Uint32(head[28:])), int(order.Uint16(head[42:])), int(order.Uint16(head[44:]))
	}
	for index := range count {
		at := offset + index*size
		if at < 0 || at+4 > len(head) {
			return false
		}
		if order.Uint32(head[at:]) == 3 {
			return true
		}
	}
	return false
}

// fileGzip is the compress magic's gzip: the name it was, when it was last modified, the
// compression, the system it came from, and the original size its last four bytes hold.
func fileGzip(head, tail []byte) string {
	little := binary.LittleEndian
	description := "gzip compressed data"
	flags := head[3]
	if flags&0x08 != 0 && flags&0x04 == 0 {
		name, _, _ := bytes.Cut(head[10:], []byte{0})
		description += fmt.Sprintf(", was \"%s\"", name)
	}
	if modified := little.Uint32(head[4:]); modified > 0 {
		description += ", last modified: " + time.Unix(int64(modified), 0).UTC().Format("Mon Jan _2 15:04:05 2006")
	}
	switch head[8] {
	case 2:
		description += ", max compression"
	case 4:
		description += ", max speed"
	}
	systems := map[byte]string{0: "FAT filesystem (MS-DOS, OS/2, NT)", 3: "Unix", 7: "Macintosh", 10: "Tops/20", 11: "NTFS filesystem (NT)", 255: ""}
	if system, known := systems[head[9]]; known && system != "" {
		description += ", from " + system
	}
	if len(tail) == 4 {
		description += fmt.Sprintf(", original size modulo 2^32 %d", little.Uint32(tail))
	}
	return description
}

// filePNG is a PNG's size, depth, colour and interlacing, from its IHDR.
func filePNG(head []byte) string {
	width, height := binary.BigEndian.Uint32(head[16:]), binary.BigEndian.Uint32(head[20:])
	depth, colour, interlace := head[24], head[25], head[28]
	colours := map[byte]string{0: "-bit grayscale", 2: "-bit/color RGB", 3: "-bit colormap", 4: "-bit gray+alpha", 6: "-bit/color RGBA"}
	interlacing := "non-interlaced"
	if interlace == 1 {
		interlacing = "interlaced"
	}
	return fmt.Sprintf("PNG image data, %d x %d, %d%s, %s", width, height, depth, colours[colour], interlacing)
}
