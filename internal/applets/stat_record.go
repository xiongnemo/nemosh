package applets

import "time"

// statRecord is a struct stat as busybox's stat prints one, the mode's type and permission bits
// spelled as Unix spells them whatever the platform keeps.
type statRecord struct {
	mode                   uint32
	size, blocks           int64
	blockSize              int64
	device, inode, links   uint64
	uid, gid               uint32
	owner, group           string
	major, minor           uint32
	access, modify, change time.Time
	// target is what a link holds.
	target string
}

// The file types of a mode, S_IFMT and the values under it.
const (
	statTypeMask  = 0o170000
	statSocket    = 0o140000
	statLink      = 0o120000
	statRegular   = 0o100000
	statBlock     = 0o060000
	statDirectory = 0o040000
	statCharacter = 0o020000
	statFIFO      = 0o010000
)

// defaultFormat is busybox's layout of a FILE (coreutils/stat.c:615-633): a device's has its
// major and minor numbers after the links.
func (s statRecord) defaultFormat(terse bool) string {
	if terse {
		return "%n %s %b %f %u %g %D %i %h %t %T %X %Y %Z %o"
	}
	links := "%h"
	if kind := s.mode & statTypeMask; kind == statBlock || kind == statCharacter {
		links = "%-5h Device type: %t,%T"
	}
	return "  File: %N\n  Size: %-10s\tBlocks: %-10b IO Block: %-6o %F\n" +
		"Device: %Dh/%dd\tInode: %-10i  Links: " + links + "\n" +
		"Access: (%04a/%10.10A)  Uid: (%5u/%8U)   Gid: (%5g/%8G)\n" +
		"Access: %x\nModify: %y\nChange: %z"
}

// conversion is busybox's print_stat (coreutils/stat.c:308-408): what a letter prints of the
// FILE, at the flags, width and precision before it. A letter it does not know prints itself.
func (s statRecord) conversion(spec cSpec, verb byte, name string) string {
	number := func(base byte, value uint64) string { return cInteger(spec, base, false, value) }
	switch verb {
	case 'n':
		return statString(spec, name)
	case 'N':
		if s.mode&statTypeMask == statLink {
			// busybox prints this one itself, and without the flags and width.
			return "'" + name + "' -> '" + s.target + "'"
		}
		return statString(spec, name)
	case 'd':
		return number('u', s.device)
	case 'D':
		return number('x', s.device)
	case 'i':
		return number('u', s.inode)
	case 'a':
		return number('o', uint64(s.mode&0o7777))
	case 'A':
		return statString(spec, statModeString(s.mode))
	case 'f':
		return number('x', uint64(s.mode))
	case 'F':
		return statString(spec, s.typeName())
	case 'h':
		return number('u', s.links)
	case 'u':
		return number('u', uint64(s.uid))
	case 'U':
		return statString(spec, s.owner)
	case 'g':
		return number('u', uint64(s.gid))
	case 'G':
		return statString(spec, s.group)
	case 't':
		return number('x', uint64(s.major))
	case 'T':
		return number('x', uint64(s.minor))
	case 's':
		return number('u', uint64(s.size))
	case 'B':
		return number('u', 512)
	case 'b':
		return number('u', uint64(s.blocks))
	case 'o':
		return number('u', uint64(s.blockSize))
	case 'x', 'y', 'z':
		return statString(spec, s.time(verb).Format("2006-01-02 15:04:05.000000000 -0700"))
	case 'X', 'Y', 'Z':
		seconds := s.time(verb + 'a' - 'A').Unix()
		if seconds < 0 {
			return cInteger(spec, 'd', true, uint64(-seconds))
		}
		return cInteger(spec, 'd', false, uint64(seconds))
	}
	return cPad(spec, "", string(verb), true)
}

// time is the access, modification or change time, by the letter of its conversion.
func (s statRecord) time(letter byte) time.Time {
	switch letter {
	case 'x':
		return s.access.Local()
	case 'y':
		return s.modify.Local()
	}
	return s.change.Local()
}

// typeName is busybox's file_type (coreutils/stat.c:123-150).
func (s statRecord) typeName() string {
	switch s.mode & statTypeMask {
	case statRegular:
		if s.size == 0 {
			return "regular empty file"
		}
		return "regular file"
	case statDirectory:
		return "directory"
	case statBlock:
		return "block special file"
	case statCharacter:
		return "character special file"
	case statFIFO:
		return "fifo"
	case statLink:
		return "symbolic link"
	case statSocket:
		return "socket"
	}
	return "weird file"
}

// statModeString is libbb's bb_mode_string (libbb/mode_string.c): the type's letter and nine
// permissions, a set-id or sticky bit in its execute place, lower case where that is set.
func statModeString(mode uint32) string {
	text := []byte{"?pc?d?b?-?l?s???"[mode>>12&0xf], '-', '-', '-', '-', '-', '-', '-', '-', '-'}
	for class, special := range [3]uint32{0o4000, 0o2000, 0o1000} {
		bits, at := mode>>(6-3*class)&7, 1+3*class
		for index := range 3 {
			if bits&(4>>index) != 0 {
				text[at+index] = "rwx"[index]
			}
		}
		if mode&special != 0 {
			text[at+2] = "SST"[class] + byte(bits&1)*('a'-'A')
		}
	}
	return string(text)
}

// statString is %s as busybox-w32's printf pads it: with zeros for the 0 flag, which C leaves
// undefined. hexdump's %s pads with blanks, as util-linux's and busybox's own printf do.
func statString(spec cSpec, text string) string {
	if spec.precision >= 0 && len(text) > spec.precision {
		text = text[:spec.precision]
	}
	return cPad(spec, "", text, true)
}
