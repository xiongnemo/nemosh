package runtime

import (
	"fmt"
	"strconv"
)

// bash mode's umask is bash 5.3's where it parts from busybox's (umask_symbolic.go), measured
// against Git for Windows' bash 5.3, which reads a mode as POSIX has chmod read one:
//   - It starts at 0022, as Git for Windows' bash does; see EnterBashMode.
//   - An operand that starts with a digit is octal, up to 07777, of which a mask keeps 0777, and
//     anything else in it is "octal number out of range", status 1. busybox refused 7777.
//   - A clause that names no class means all three, unfiltered: under 0124 `umask =rx` is 0222
//     and `+r` 0120, where busybox filters both through the mask.
//   - u, g and o as permissions copy that class as the operand found it, the last one written
//     standing for the rest before it: under 0124 `a=u` is 0111 and `a=,a=u` too. busybox
//     copied what `=` had just cleared.
//   - X is x when the operand found some class with x, so `a=X` under 0124 is 0666, and
//     `u+x,g+X` under 0111 is 0011; busybox looks at the mode as it stands, and made them 0777
//     and 0001. s and t are taken and left out, since a mask holds neither; busybox refused them.
//   - A clause that is empty, a missing operator or a stray character is refused in bash's
//     words, the character it stopped at quoted -- a NUL at the operand's end, as bash prints
//     it -- status 1; busybox skipped an empty clause and said "illegal mode", status 2.
//   - -S with an operand prints the new mask as it says one.

// setBashUmask sets the mask from operand as bash does, and answers the status.
func (r Runtime) setBashUmask(operand string, symbolic bool) int {
	var mask uint16
	if operand != "" && operand[0] >= '0' && operand[0] <= '9' {
		value, err := strconv.ParseUint(operand, 8, 16)
		if err != nil || value > 0o7777 {
			fmt.Fprintf(r.streams.Stderr, "%sumask: %s: octal number out of range\n", r.diagnosticPrefix(), operand)
			return 1
		}
		mask = uint16(value) & 0o777
	} else {
		mode, err := bashSymbolicMode(^r.mask.value&0o777, operand)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "%sumask: %v\n", r.diagnosticPrefix(), err)
			return 1
		}
		mask = ^mode & 0o777
	}
	r.mask.set(mask)
	if symbolic {
		fmt.Fprintln(r.streams.Stdout, symbolicMask(mask))
	}
	return 0
}

// bashSymbolicMode applies a symbolic mode to the permissions a mask leaves, initial, as bash's
// parse_symbolic_mode does: clauses split at commas, each its class letters and then one action
// or more, an operator and the permissions it adds, takes away or sets.
func bashSymbolicMode(initial uint16, spec string) (uint16, error) {
	mode := initial
	for {
		var who uint16
		for spec != "" && bashModeClass(spec[0]) != 0 {
			who |= bashModeClass(spec[0])
			spec = spec[1:]
		}
		for {
			operator := byte(0)
			if spec != "" {
				operator = spec[0]
			}
			if operator != '+' && operator != '-' && operator != '=' {
				return 0, fmt.Errorf("`%c': invalid symbolic mode operator", operator)
			}
			var permissions uint16
			permissions, spec = bashModePermissions(initial, spec[1:])
			mode = bashModeAction(mode, who, operator, permissions)
			if spec == "" {
				return mode, nil
			}
			if spec[0] == ',' {
				spec = spec[1:]
				break
			}
			if spec[0] != '+' && spec[0] != '-' && spec[0] != '=' {
				return 0, fmt.Errorf("`%c': invalid symbolic mode character", spec[0])
			}
		}
	}
}

// bashModeClass is the permissions a class letter names, or 0 for a letter that is not one.
func bashModeClass(letter byte) uint16 {
	switch letter {
	case 'u':
		return 0o700
	case 'g':
		return 0o070
	case 'o':
		return 0o007
	case 'a':
		return 0o777
	}
	return 0
}

// bashModePermissions reads an action's permissions, in every class, and answers the rest of
// the clause: letters, X as x when the operand found some class with x, s and t as nothing, and
// a class to copy as the operand found it, which stands for whatever was written before it.
func bashModePermissions(initial uint16, spec string) (uint16, string) {
	var permissions uint16
	for ; spec != ""; spec = spec[1:] {
		switch letter := spec[0]; letter {
		case 'r':
			permissions |= 0o444
		case 'w':
			permissions |= 0o222
		case 'x':
			permissions |= 0o111
		case 'X':
			if initial&0o111 != 0 {
				permissions |= 0o111
			}
		case 's', 't':
		case 'u', 'g', 'o':
			bits := initial & bashModeClass(letter)
			bits |= bits>>3 | bits>>6 | bits<<3 | bits<<6
			permissions = bits & 0o777
		default:
			return permissions, spec
		}
	}
	return permissions, spec
}

// bashModeAction applies one action to mode: permissions limited to who when the clause names a
// class, and `=` clearing who, or everything when it names none.
func bashModeAction(mode, who uint16, operator byte, permissions uint16) uint16 {
	if who != 0 {
		permissions &= who
	}
	switch operator {
	case '+':
		return mode | permissions
	case '-':
		return mode &^ permissions
	}
	if who == 0 {
		who = 0o777
	}
	return mode&^who | permissions
}
