package runtime

import (
	"errors"
	"strings"
)

// umask's symbolic forms, busybox's: `umask -S` prints the mask as the permissions it leaves,
// `u=rwx,g=rx,o=rx`, and an operand may be written that way too -- `umask u=rwx,g=rx,o=`,
// `umask g-w`, `umask a+r`. Both were "invalid mask".
//
// The operand is read as chmod reads a mode, and applied to the permissions the mask leaves.
// The rules are busybox's, measured, and where bash differs busybox is followed:
//   - A clause with no class letters is filtered through the mask, as POSIX has chmod
//     filter it through the umask: under 0124, `umask =rx` is 0326. bash makes it 0222.
//   - An empty clause is skipped, so `u-r,,u-r` is `u-r,u-r`. bash refuses it.
//   - `u=g` copies g's permissions as they stand once `=` has cleared u's, so `a=u` clears
//     everything.
//   - X is x, but only when some class has x at that point.
//   - A special bit cannot be held in a mask, so s and t are refused wherever they would be
//     set: with u, g or a, or with no class letters. o carries neither.

var errIllegalMode = errors.New("illegal mode")

// umaskClasses are the bits a class letter stands for: its permissions, with set-user-id for
// u, set-group-id for g, and every special bit for a.
var umaskClasses = map[byte]uint32{'u': 0o4700, 'g': 0o2070, 'o': 0o0007, 'a': 0o7777}

// umaskPermissions are the permission letters, as bits in every class.
var umaskPermissions = map[byte]uint32{'r': 0o444, 'w': 0o222, 'x': 0o111, 'X': 0o111, 's': 0o6000, 't': 0o1000}

// symbolicMask is the mask as `umask -S` prints it.
func symbolicMask(mask uint16) string {
	allowed := ^mask & 0o777
	parts := make([]string, 0, 3)
	for _, class := range []struct {
		name  string
		shift uint
	}{{"u", 6}, {"g", 3}, {"o", 0}} {
		bits := allowed >> class.shift & 7
		letters := ""
		for _, permission := range []struct {
			bit    uint16
			letter string
		}{{4, "r"}, {2, "w"}, {1, "x"}} {
			if bits&permission.bit != 0 {
				letters += permission.letter
			}
		}
		parts = append(parts, class.name+"="+letters)
	}
	return strings.Join(parts, ",")
}

// applySymbolicMask applies a symbolic operand to a mask.
func applySymbolicMask(mask uint16, spec string) (uint16, error) {
	mode := uint32(^mask & 0o777)
	for _, clause := range strings.Split(spec, ",") {
		if clause == "" {
			continue
		}
		var err error
		if mode, err = applyModeClause(mode, uint32(mask), clause); err != nil {
			return 0, err
		}
	}
	if mode > 0o777 {
		return 0, errIllegalMode
	}
	return uint16(^mode & 0o777), nil
}

// applyModeClause applies one clause, `ug+w-x` or `o=u`: its class letters, then each action.
func applyModeClause(mode, mask uint32, clause string) (uint32, error) {
	var who uint32
	for len(clause) > 0 && umaskClasses[clause[0]] != 0 {
		who |= umaskClasses[clause[0]]
		clause = clause[1:]
	}
	if clause == "" {
		return 0, errIllegalMode
	}
	for clause != "" {
		operator := clause[0]
		if operator != '+' && operator != '-' && operator != '=' {
			return 0, errIllegalMode
		}
		clause = clause[1:]
		if operator == '=' {
			if who == 0 {
				mode = 0
			} else {
				mode &^= who
			}
		}
		var bits uint32
		bits, clause = actionBits(mode, clause)
		if who == 0 {
			bits &^= mask
		} else {
			bits &= who
		}
		if operator == '-' {
			mode &^= bits
		} else {
			mode |= bits
		}
	}
	return mode, nil
}

// actionBits reads what one action adds or takes away: a class to copy, or permission
// letters. It returns the bits, in every class, and the rest of the clause.
func actionBits(mode uint32, clause string) (uint32, string) {
	if len(clause) > 0 && strings.IndexByte("ugo", clause[0]) >= 0 {
		shift := map[byte]uint{'u': 6, 'g': 3, 'o': 0}[clause[0]]
		bits := mode >> shift & 7
		return bits<<6 | bits<<3 | bits, clause[1:]
	}
	var bits uint32
	for len(clause) > 0 && umaskPermissions[clause[0]] != 0 {
		if clause[0] != 'X' || mode&0o111 != 0 {
			bits |= umaskPermissions[clause[0]]
		}
		clause = clause[1:]
	}
	return bits, clause
}
