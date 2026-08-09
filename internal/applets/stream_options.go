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
	for index, arg := range args {
		if arg == "--" {
			return args[index+1:], nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			return args[index:], nil
		}
		if containsString(supported, arg) {
			continue
		}
		return nil, unsupportedStreamOption(applet, arg, supported)
	}
	return nil, nil
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
