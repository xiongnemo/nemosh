package applets

import (
	"os"
	"strconv"
	"strings"
)

// chmodClasses are the bits a class letter stands for: its permissions, with set-user-id for
// u, set-group-id for g, and every special bit for a.
var chmodClasses = map[byte]uint32{'a': 0o7777, 'u': 0o4700, 'g': 0o2070, 'o': 0o0007}

// chmodPermissions are the permission letters, as bits in every class. X is x, but only for a
// directory or a file some class may already run.
var chmodPermissions = map[byte]uint32{'r': 0o444, 'w': 0o222, 'x': 0o111, 'X': 0o111, 's': 0o6000, 't': 0o1000}

// applyChmodMode is busybox's bb_parse_mode (libbb/parse_mode.c). An octal MODE, up to 7777,
// is the new mode outright. Otherwise MODE is clauses separated by commas, each class letters
// [ugoa] and then actions: +, - or =, each followed by permission letters [rwxXst] or by one
// class [ugo] whose permissions it copies. A clause with no class letters is filtered through
// the umask. An empty clause, or an action with no letters, changes nothing. ok is false for a
// MODE that is neither, as busybox refuses it: `u`, `8`, `u+q`.
func applyChmodMode(spec string, current, umask uint32, isDir bool) (uint32, bool) {
	if spec != "" && spec[0] >= '0' && spec[0] <= '7' {
		value, err := strconv.ParseUint(spec, 8, 32)
		return uint32(value), err == nil && value <= 0o7777
	}
	mode := current
	for spec != "" {
		if spec[0] == ',' {
			spec = spec[1:]
			continue
		}
		var who uint32
		for spec != "" && chmodClasses[spec[0]] != 0 {
			who |= chmodClasses[spec[0]]
			if spec = spec[1:]; spec == "" {
				return 0, false
			}
		}
		for {
			operator := spec[0]
			if operator != '+' && operator != '-' && operator != '=' {
				return 0, false
			}
			if operator == '=' {
				if who == 0 {
					mode &^= 0o7777
				} else {
					mode &^= who
				}
			}
			var bits uint32
			bits, spec = chmodActionBits(mode, isDir, spec[1:])
			if who == 0 {
				bits &^= umask
			} else {
				bits &= who
			}
			if operator == '-' {
				mode &^= bits
			} else {
				mode |= bits
			}
			if spec == "" || spec[0] == ',' {
				break
			}
		}
	}
	return mode, true
}

// chmodActionBits reads what one action adds or takes away, a class to copy or permission
// letters, and returns the bits in every class with the rest of the clause.
func chmodActionBits(mode uint32, isDir bool, spec string) (uint32, string) {
	if spec != "" && strings.IndexByte("ugo", spec[0]) >= 0 {
		copied := chmodClasses[spec[0]] & 0o777 & mode
		var bits uint32
		for _, permission := range []uint32{0o444, 0o222, 0o111} {
			if copied&permission != 0 {
				bits |= permission
			}
		}
		return bits, spec[1:]
	}
	var bits uint32
	for spec != "" && chmodPermissions[spec[0]] != 0 {
		if spec[0] != 'X' || isDir || mode&0o111 != 0 {
			bits |= chmodPermissions[spec[0]]
		}
		spec = spec[1:]
	}
	return bits, spec
}

// chmodModeLetters is the nine permission letters -v prints, as busybox's bb_mode_string
// writes them: s or S where set-user-id or set-group-id is, t or T for the sticky bit,
// lowercase when the x under it is set.
func chmodModeLetters(mode uint32) string {
	letters := []byte("rwxrwxrwx")
	for index := range letters {
		if mode&(0o400>>index) == 0 {
			letters[index] = '-'
		}
	}
	for _, special := range []struct {
		bit   uint32
		index int
		shown byte
	}{{0o4000, 2, 's'}, {0o2000, 5, 's'}, {0o1000, 8, 't'}} {
		if mode&special.bit != 0 {
			if letters[special.index] == '-' {
				letters[special.index] = special.shown - 'a' + 'A'
			} else {
				letters[special.index] = special.shown
			}
		}
	}
	return string(letters)
}

// fileModeOfBits is a mode's twelve bits as Go's FileMode, whose special bits are flags of its
// own and not the octal 7000 above the permissions.
func fileModeOfBits(bits uint32) os.FileMode {
	mode := os.FileMode(bits & 0o777)
	if bits&0o4000 != 0 {
		mode |= os.ModeSetuid
	}
	if bits&0o2000 != 0 {
		mode |= os.ModeSetgid
	}
	if bits&0o1000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

// bitsOfFileMode is the reverse: Go's FileMode as the twelve bits chmod reckons in.
func bitsOfFileMode(mode os.FileMode) uint32 {
	bits := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		bits |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		bits |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		bits |= 0o1000
	}
	return bits
}

// fileModeMaskView is a view that holds the shell's umask, which chmod filters a MODE with no
// class letters through.
type fileModeMaskView interface {
	FileModeMask() uint16
}

// processFileModeMask is the umask of the shell an applet runs in, or 022 outside one.
func processFileModeMask(view ProcessView) uint32 {
	if holder, ok := view.(fileModeMaskView); ok {
		return uint32(holder.FileModeMask())
	}
	return 0o022
}
