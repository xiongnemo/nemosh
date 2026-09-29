package runtime

import (
	"fmt"
	"strconv"
)

type fileModeMask struct {
	value uint16
}

// newFileModeMask is a new shell's umask: on Linux and macOS the process's own, which it was
// given, as every shell starts from; on Windows, which gives none, 0022. It was 0022
// everywhere, so on Linux `umask` said 0022 whatever the process had.
func newFileModeMask() *fileModeMask {
	return &fileModeMask{value: initialFileModeMask}
}

// FileModeMask is the umask, for the applets that read one: chmod filters a MODE with no class
// letters through it.
func (r Runtime) FileModeMask() uint16 {
	if r.mask == nil {
		return initialFileModeMask
	}
	return r.mask.value
}

// umask is the POSIX builtin: with no operand it prints the mask in octal; with one it sets
// it, in octal or in the symbolic form, `u=rwx,g=rx,o=`. -S prints the mask symbolically, as
// the permissions it leaves, and with an operand sets it quietly. All as busybox's, which
// reads the first operand and no other, and refuses any other option; see umask_symbolic.go
// for the symbolic half, which was "invalid mask". -p is bash's: the mask printed as the
// command that sets it.
func (r Runtime) umask(args []string) int {
	symbolic, reusable := false, false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for _, letter := range option[1:] {
			switch letter {
			case 'S':
				symbolic = true
			case 'p':
				reusable = true
			default:
				// `umask -rwx` too, which reads as options before it can read as a mode.
				fmt.Fprintf(r.streams.Stderr, "umask: illegal option -%c\n", letter)
				return 2
			}
		}
	}
	if len(args) == 0 {
		r.printUmask(symbolic, reusable)
		return 0
	}
	mask, err := parseFileModeMask(args[0])
	if err != nil {
		mask, err = applySymbolicMask(r.mask.value, args[0])
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "umask: illegal mode: %s\n", args[0])
		return 2
	}
	r.mask.value = mask
	return 0
}

func (r Runtime) printUmask(symbolic, reusable bool) {
	mask := fmt.Sprintf("%04o", r.mask.value)
	if symbolic {
		mask = symbolicMask(r.mask.value)
	}
	switch {
	case reusable && symbolic:
		fmt.Fprintf(r.streams.Stdout, "umask -S %s\n", mask)
	case reusable:
		fmt.Fprintf(r.streams.Stdout, "umask %s\n", mask)
	default:
		fmt.Fprintln(r.streams.Stdout, mask)
	}
}

func parseFileModeMask(arg string) (uint16, error) {
	mask, err := strconv.ParseUint(arg, 8, 16)
	if err != nil || mask > 0o777 {
		return 0, strconv.ErrSyntax
	}
	return uint16(mask), nil
}
