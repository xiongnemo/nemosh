package applets

import (
	"fmt"
	"strings"
)

// streamOperands separates the operands of a stream applet from its options,
// and refuses any option the applet does not implement.
//
// The refusal is the point. These operands used to fall through to the file
// opener, so `cat -n f.txt` answered `cannot open '-n': No such file or
// directory` -- loud, but naming the wrong cause, which sends the reader after
// a file that was never meant to be one. `supported` lists the spellings the
// caller really handles and has already consumed.
//
// A `--` ends option parsing and a lone `-` is an operand, which is what getopt
// does and what busybox inherits from it.
func streamOperands(applet string, args []string, supported ...string) ([]string, error) {
	_, paths, err := streamOptionsAndOperands(applet, args, supported...)
	return paths, err
}

// streamOptionsAndOperands is the same walk, but reports which of the supported
// spellings were actually given. A caller that merely tolerates its options can
// use streamOperands and ignore them; one that acts on them needs to know.
func streamOptionsAndOperands(applet string, args []string, supported ...string) (given []string, paths []string, err error) {
	for index, arg := range args {
		if arg == "--" {
			return given, args[index+1:], nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			return given, args[index:], nil
		}
		if containsString(supported, arg) {
			given = append(given, arg)
			continue
		}
		return nil, nil, unsupportedStreamOption(applet, arg, supported)
	}
	return given, nil, nil
}

func unsupportedStreamOption(applet, arg string, supported []string) error {
	if len(supported) == 0 {
		return fmt.Errorf("unsupported %s option: %s", applet, arg)
	}
	return fmt.Errorf("unsupported %s option: %s; this build implements %s", applet, arg, strings.Join(supported, ", "))
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
