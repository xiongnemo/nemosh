package runtime

import (
	"fmt"
	"strconv"
)

type fileModeMask struct {
	value uint16
}

func newFileModeMask() *fileModeMask {
	return &fileModeMask{value: 0o022}
}

func (r Runtime) umask(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(r.streams.Stdout, "%04o\n", r.mask.value)
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(r.streams.Stderr, "umask: expected at most one operand")
		return 2
	}
	mask, err := parseFileModeMask(args[0])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "umask: %s: invalid mask\n", args[0])
		return 2
	}
	r.mask.value = mask
	return 0
}

func parseFileModeMask(arg string) (uint16, error) {
	mask, err := strconv.ParseUint(arg, 8, 16)
	if err != nil || mask > 0o777 {
		return 0, strconv.ErrSyntax
	}
	return uint16(mask), nil
}
